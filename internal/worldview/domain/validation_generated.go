// Code generated from the frozen worldview request schemas. DO NOT EDIT.
package domain

import (
	"InkLore/internal/platform/scope"
	"encoding/json"
)

var _ = scope.ValidID

func (v *CreateCharacter) UnmarshalJSON(raw []byte) error {
	if err := requiredObject(raw, []string{"name", "description", "aliases", "role_type"}, []string{}); err != nil {
		return err
	}
	type plain CreateCharacter
	return json.Unmarshal(raw, (*plain)(v))
}
func (v CreateCharacter) Validate() error {
	if !Text(v.Name, 1, 60) {
		return Invalid()
	}
	if !Text(v.Description, 0, 20000) {
		return Invalid()
	}
	if v.Aliases == nil || len(v.Aliases) > 20 {
		return Invalid()
	}
	for _, x := range v.Aliases {
		if !Text(x, 1, 60) {
			return Invalid()
		}
	}
	if !One(v.RoleType, "protagonist", "supporting", "antagonist", "other") {
		return Invalid()
	}
	return nil
}
func (v *UpdateCharacter) UnmarshalJSON(raw []byte) error {
	if err := requiredObject(raw, []string{"expected_revision", "name", "description", "aliases", "role_type", "status"}, []string{}); err != nil {
		return err
	}
	type plain UpdateCharacter
	return json.Unmarshal(raw, (*plain)(v))
}
func (v UpdateCharacter) Validate() error {
	if v.ExpectedRevision < 1 || v.ExpectedRevision > MaxRevision {
		return Malformed()
	}
	if !Text(v.Name, 1, 60) {
		return Invalid()
	}
	if !Text(v.Description, 0, 20000) {
		return Invalid()
	}
	if v.Aliases == nil || len(v.Aliases) > 20 {
		return Invalid()
	}
	for _, x := range v.Aliases {
		if !Text(x, 1, 60) {
			return Invalid()
		}
	}
	if !One(v.RoleType, "protagonist", "supporting", "antagonist", "other") {
		return Invalid()
	}
	if !One(v.Status, "active", "archived") {
		return Invalid()
	}
	return nil
}
func (v *CreateLore) UnmarshalJSON(raw []byte) error {
	if err := requiredObject(raw, []string{"title", "description", "category"}, []string{}); err != nil {
		return err
	}
	type plain CreateLore
	return json.Unmarshal(raw, (*plain)(v))
}
func (v CreateLore) Validate() error {
	if !Text(v.Title, 1, 120) {
		return Invalid()
	}
	if !Text(v.Description, 0, 20000) {
		return Invalid()
	}
	if !One(v.Category, "faction", "place", "system", "item", "history", "other") {
		return Invalid()
	}
	return nil
}
func (v *UpdateLore) UnmarshalJSON(raw []byte) error {
	if err := requiredObject(raw, []string{"expected_revision", "title", "description", "category", "status"}, []string{}); err != nil {
		return err
	}
	type plain UpdateLore
	return json.Unmarshal(raw, (*plain)(v))
}
func (v UpdateLore) Validate() error {
	if v.ExpectedRevision < 1 || v.ExpectedRevision > MaxRevision {
		return Malformed()
	}
	if !Text(v.Title, 1, 120) {
		return Invalid()
	}
	if !Text(v.Description, 0, 20000) {
		return Invalid()
	}
	if !One(v.Category, "faction", "place", "system", "item", "history", "other") {
		return Invalid()
	}
	if !One(v.Status, "active", "archived") {
		return Invalid()
	}
	return nil
}
func (v *CreateRelation) UnmarshalJSON(raw []byte) error {
	if err := requiredObject(raw, []string{"subject_id", "predicate", "object_id", "description", "weight"}, []string{}); err != nil {
		return err
	}
	type plain CreateRelation
	return json.Unmarshal(raw, (*plain)(v))
}
func (v CreateRelation) Validate() error {
	if !scope.ValidID(v.SubjectID) {
		return Invalid()
	}
	if !Text(v.Predicate, 1, 60) {
		return Invalid()
	}
	if !scope.ValidID(v.ObjectID) {
		return Invalid()
	}
	if !Text(v.Description, 0, 2000) {
		return Invalid()
	}
	if v.Weight < 0 || v.Weight > 100 {
		return Invalid()
	}
	return nil
}
func (v *UpdateRelation) UnmarshalJSON(raw []byte) error {
	if err := requiredObject(raw, []string{"expected_revision", "subject_id", "predicate", "object_id", "description", "weight", "status"}, []string{}); err != nil {
		return err
	}
	type plain UpdateRelation
	return json.Unmarshal(raw, (*plain)(v))
}
func (v UpdateRelation) Validate() error {
	if v.ExpectedRevision < 1 || v.ExpectedRevision > MaxRevision {
		return Malformed()
	}
	if !scope.ValidID(v.SubjectID) {
		return Invalid()
	}
	if !Text(v.Predicate, 1, 60) {
		return Invalid()
	}
	if !scope.ValidID(v.ObjectID) {
		return Invalid()
	}
	if !Text(v.Description, 0, 2000) {
		return Invalid()
	}
	if v.Weight < 0 || v.Weight > 100 {
		return Invalid()
	}
	if !One(v.Status, "active", "retired") {
		return Invalid()
	}
	return nil
}
func (v *RetractFact) UnmarshalJSON(raw []byte) error {
	if err := requiredObject(raw, []string{"expected_revision", "expected_world_revision", "reason"}, []string{}); err != nil {
		return err
	}
	type plain RetractFact
	return json.Unmarshal(raw, (*plain)(v))
}
func (v RetractFact) Validate() error {
	if v.ExpectedRevision < 1 || v.ExpectedRevision > MaxRevision {
		return Malformed()
	}
	if v.ExpectedWorldRevision < 1 || v.ExpectedWorldRevision > MaxRevision {
		return Malformed()
	}
	if !Text(v.Reason, 1, 1000) {
		return Invalid()
	}
	return nil
}
func (v *SupersedeFact) UnmarshalJSON(raw []byte) error {
	if err := requiredObject(raw, []string{"expected_revision", "expected_world_revision", "value", "resolution", "reason"}, []string{}); err != nil {
		return err
	}
	type plain SupersedeFact
	return json.Unmarshal(raw, (*plain)(v))
}
func (v SupersedeFact) Validate() error {
	if v.ExpectedRevision < 1 || v.ExpectedRevision > MaxRevision {
		return Malformed()
	}
	if v.ExpectedWorldRevision < 1 || v.ExpectedWorldRevision > MaxRevision {
		return Malformed()
	}
	if err := v.Value.Validate(); err != nil {
		return err
	}
	if !One(v.Resolution, "correction", "retcon") {
		return Invalid()
	}
	if !Text(v.Reason, 1, 1000) {
		return Invalid()
	}
	return nil
}
func (v *CheckFacts) UnmarshalJSON(raw []byte) error {
	if err := requiredObject(raw, []string{"expected_world_revision", "fact_proposals"}, []string{}); err != nil {
		return err
	}
	type plain CheckFacts
	return json.Unmarshal(raw, (*plain)(v))
}
func (v CheckFacts) Validate() error {
	if v.ExpectedWorldRevision < 1 || v.ExpectedWorldRevision > MaxRevision {
		return Malformed()
	}
	if v.FactProposals == nil || len(v.FactProposals) > 100 {
		return Invalid()
	}
	for _, x := range v.FactProposals {
		if err := x.Validate(); err != nil {
			return err
		}
	}
	return nil
}
func (v *RevisionCommand) UnmarshalJSON(raw []byte) error {
	if err := requiredObject(raw, []string{"expected_revision"}, []string{}); err != nil {
		return err
	}
	type plain RevisionCommand
	return json.Unmarshal(raw, (*plain)(v))
}
func (v RevisionCommand) Validate() error {
	if v.ExpectedRevision < 1 || v.ExpectedRevision > MaxRevision {
		return Malformed()
	}
	return nil
}
func (v *FactProposal) UnmarshalJSON(raw []byte) error {
	if err := requiredObject(raw, []string{"subject_id", "predicate", "fact_key", "value", "action", "target_event_id", "resolution", "reason"}, []string{"value", "target_event_id", "reason"}); err != nil {
		return err
	}
	type plain FactProposal
	return json.Unmarshal(raw, (*plain)(v))
}
func (v FactProposal) Validate() error {
	if !scope.ValidID(v.SubjectID) {
		return Invalid()
	}
	if !One(v.Predicate, "life_status", "realm_rank", "item_status", "location", "relationship", "custom") {
		return Invalid()
	}
	if !Text(v.FactKey, 1, 120) {
		return Invalid()
	}
	if !factKeyPattern.MatchString(v.FactKey) {
		return Invalid()
	}
	if v.Value != nil {
		if err := v.Value.Validate(); err != nil {
			return err
		}
	}
	if !One(v.Action, "assert", "supersede", "retract") {
		return Invalid()
	}
	if v.TargetEventID != nil && !scope.ValidID(*v.TargetEventID) {
		return Invalid()
	}
	if !One(v.Resolution, "none", "correction", "retcon") {
		return Invalid()
	}
	if v.Reason != nil && !Text(*v.Reason, 1, 1000) {
		return Invalid()
	}
	return nil
}
