package types

import "encoding/xml"

// CASAuthURLResponse is returned when the frontend requests a CAS login URL.
type CASAuthURLResponse struct {
	Success             bool   `json:"success"`
	ProviderDisplayName string `json:"provider_display_name"`
	AuthorizationURL    string `json:"authorization_url"`
}

// CASSettings holds the parsed CAS configuration consumed by the auth handler
// and service layer.
type CASSettings struct {
	Enabled              bool
	ServerURL            string
	ProviderDisplayName  string
	CallbackURL          string
	UsernameAttribute    string // CAS attribute → WeKnora username (empty = <cas:user>)
	EmailAttribute       string // CAS attribute → WeKnora email (empty = none, use placeholder)
	DisplayNameAttribute string // CAS attribute → WeKnora display name (empty = username)
}

// CASUserInfo holds the user attributes extracted from a CAS validation response.
type CASUserInfo struct {
	Username    string
	DisplayName string
	Email       string
}

// ── CAS 3.0 XML parsing (/p3/serviceValidate) ──

type casServiceResponse struct {
	XMLName               xml.Name         `xml:"serviceResponse"`
	AuthenticationSuccess *casAuthSuccess  `xml:"authenticationSuccess"`
	AuthenticationFailure *casAuthFailure  `xml:"authenticationFailure"`
}

type casAuthSuccess struct {
	User       string   `xml:"user"`
	Attributes casAttrs `xml:"attributes"`
}

type casAuthFailure struct {
	Code        string `xml:"code,attr"`
	Description string `xml:",chardata"`
}

// casAttrs collects every child element as a key-value pair so attribute
// mapping can be configured via env vars instead of struct tags.
type casAttrs map[string]string

func (a *casAttrs) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	if *a == nil {
		*a = make(casAttrs)
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var val string
			if err := d.DecodeElement(&val, &t); err != nil {
				return err
			}
			(*a)[t.Name.Local] = val
		case xml.EndElement:
			return nil
		}
	}
}

// ParseCASServiceResponse parses raw CAS XML and returns user info or an
// auth-failure description. When the ticket is invalid CASUserInfo is nil
// and the string contains the CAS error message.
func ParseCASServiceResponse(raw []byte) (*CASUserInfo, string) {
	var resp casServiceResponse
	if err := xml.Unmarshal(raw, &resp); err != nil {
		return nil, "failed to parse CAS response"
	}
	if resp.AuthenticationSuccess != nil {
		info := &CASUserInfo{
			Username: resp.AuthenticationSuccess.User,
		}
		if resp.AuthenticationSuccess.Attributes != nil {
			info.DisplayName = resp.AuthenticationSuccess.Attributes["displayName"]
			info.Email = resp.AuthenticationSuccess.Attributes["email"]
			if info.Email == "" {
				info.Email = resp.AuthenticationSuccess.Attributes["mail"]
			}
		}
		return info, ""
	}
	if resp.AuthenticationFailure != nil {
		return nil, resp.AuthenticationFailure.Description
	}
	return nil, "unknown CAS response format"
}
