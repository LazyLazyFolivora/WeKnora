package service

import (
	"fmt"
	"strings"
)

// Allowed entity types for graph import.
var allowedGraphEntityTypes = map[string]struct{}{
	"Company":            {},
	"Drug":               {},
	"Target":             {},
	"Indication":         {},
	"ClinicalTrial":      {},
	"DealEvent":          {},
	"DealItem":           {},
	"ApprovalEvent":      {},
	"Policy":             {},
	"TCMFormula":         {},
	"Compound":           {},
	"Pathway":            {},
	"DevelopmentProject": {},
	"TrialIndication":    {},
}

// Allowed relation types for graph import.
var allowedGraphRelationTypes = map[string]struct{}{
	"DEVELOPS":                 {},
	"PARTICIPATES_IN":          {},
	"INVESTED_IN":              {},
	"SUBSIDIARY_OF":            {},
	"SPONSORS":                 {},
	"ISSUES":                   {},
	"APPROVES":                 {},
	"PARTICIPATES_IN_PROJECT":  {},
	"TARGETS":                  {},
	"TREATS":                   {},
	"IN_TRIAL":                 {},
	"HAS_APPROVAL":             {},
	"HAS_ITEM":                 {},
	"INVOLVES_DRUG":            {},
	"EVALUATES":                {},
	"FOR_INDICATION":           {},
	"IN_PATHWAY":               {},
	"ASSOCIATED_WITH":          {},
	"HOMOLOG_OF":               {},
	"SUBTYPE_OF":               {},
	"AFFECTS":                  {},
	"DEVELOPED_BY":             {},
	"CONTAINS":                 {},
}

// graphDir records a single allowed direction for a relation type.
type graphDir struct{ from, to string }

// Relation direction matrix: which source→target entity type pairs each relation type allows.
var allowedGraphDirections = map[string][]graphDir{
	"DEVELOPS":                 {{"Company", "Drug"}, {"DevelopmentProject", "Drug"}},
	"PARTICIPATES_IN":          {{"Company", "DealEvent"}},
	"INVESTED_IN":              {{"Company", "Company"}},
	"SUBSIDIARY_OF":            {{"Company", "Company"}},
	"SPONSORS":                 {{"Company", "ClinicalTrial"}},
	"ISSUES":                   {{"Company", "Policy"}},
	"APPROVES":                 {{"Company", "ApprovalEvent"}},
	"PARTICIPATES_IN_PROJECT":  {{"Company", "DevelopmentProject"}},
	"TARGETS":                  {{"Drug", "Target"}, {"Compound", "Target"}},
	"TREATS":                   {{"Drug", "Indication"}, {"TCMFormula", "Indication"}},
	"IN_TRIAL":                 {{"Drug", "ClinicalTrial"}},
	"HAS_APPROVAL":             {{"Drug", "ApprovalEvent"}},
	"HAS_ITEM":                 {{"DealEvent", "DealItem"}},
	"INVOLVES_DRUG":            {{"DealItem", "Drug"}},
	"EVALUATES":                {{"ClinicalTrial", "TrialIndication"}},
	"FOR_INDICATION":           {{"TrialIndication", "Indication"}},
	"IN_PATHWAY":               {{"Target", "Pathway"}},
	"ASSOCIATED_WITH":          {{"Target", "Indication"}},
	"HOMOLOG_OF":               {{"Target", "Target"}},
	"SUBTYPE_OF":               {{"Indication", "Indication"}},
	"AFFECTS":                  {{"Policy", "Company"}, {"Policy", "Drug"}, {"Policy", "Indication"}, {"Policy", "Target"}},
	"DEVELOPED_BY":             {{"TCMFormula", "Company"}},
	"CONTAINS":                 {{"TCMFormula", "Compound"}},
}

// validateEntityType returns an error if entityType is not in the allowed list.
func validateEntityType(entityType string) error {
	if _, ok := allowedGraphEntityTypes[entityType]; !ok {
		return fmt.Errorf("未知实体类型 %q，允许的类型: %s", entityType, allowedEntityTypeNames())
	}
	return nil
}

// validateRelationType returns an error if relationType is not in the allowed list.
func validateRelationType(relationType string) error {
	if _, ok := allowedGraphRelationTypes[relationType]; !ok {
		return fmt.Errorf("未知关系类型 %q", relationType)
	}
	return nil
}

// validateRelationDirection checks that the (fromType, relationType, toType) triple is allowed.
func validateRelationDirection(fromType, relationType, toType string) error {
	dirs, ok := allowedGraphDirections[relationType]
	if !ok {
		return fmt.Errorf("未知关系类型 %q", relationType)
	}
	for _, d := range dirs {
		if d.from == fromType && d.to == toType {
			return nil
		}
	}
	return fmt.Errorf("关系方向 %q -[%s]-> %q 不允许", fromType, relationType, toType)
}

// SanitizeRelationType checks that the given type is in the allowed set.
// When returning nil, the type is safe to interpolate into Cypher.
func SanitizeRelationType(relationType string) error {
	// Must match exactly — no whitespace, no special chars.
	if _, ok := allowedGraphRelationTypes[relationType]; ok {
		return nil
	}
	return fmt.Errorf("关系类型 %q 不在白名单中", relationType)
}

func allowedEntityTypeNames() string {
	names := make([]string, 0, len(allowedGraphEntityTypes))
	for k := range allowedGraphEntityTypes {
		names = append(names, k)
	}
	return strings.Join(names, ", ")
}
