// Code generated from the frozen worldview operation catalog. DO NOT EDIT.
package transport

type route struct {
	method, path, operation string
	query                   []string
}

var routes = []route{
	{"GET", "/v1/worldview/novels/:novel_id/characters", "listcharacters", []string{"limit", "cursor", "deleted"}},
	{"POST", "/v1/worldview/novels/:novel_id/characters", "createCharacter", []string{}},
	{"GET", "/v1/worldview/characters/:character_id", "getCharacter", []string{}},
	{"PUT", "/v1/worldview/characters/:character_id", "updateCharacter", []string{}},
	{"DELETE", "/v1/worldview/characters/:character_id", "deleteCharacter", []string{"expected_revision"}},
	{"GET", "/v1/worldview/novels/:novel_id/lore", "listlore", []string{"limit", "cursor", "deleted"}},
	{"POST", "/v1/worldview/novels/:novel_id/lore", "createLore", []string{}},
	{"GET", "/v1/worldview/lore/:lore_id", "getLore", []string{}},
	{"PUT", "/v1/worldview/lore/:lore_id", "updateLore", []string{}},
	{"DELETE", "/v1/worldview/lore/:lore_id", "deleteLore", []string{"expected_revision"}},
	{"POST", "/v1/worldview/characters/:character_id/restore", "restoreCharacter", []string{}},
	{"POST", "/v1/worldview/lore/:lore_id/restore", "restoreLore", []string{}},
	{"GET", "/v1/worldview/novels/:novel_id/relations", "listrelations", []string{"limit", "cursor", "deleted"}},
	{"POST", "/v1/worldview/novels/:novel_id/relations", "createRelation", []string{}},
	{"GET", "/v1/worldview/relations/:relation_id", "getRelation", []string{}},
	{"PUT", "/v1/worldview/relations/:relation_id", "updateRelation", []string{}},
	{"DELETE", "/v1/worldview/relations/:relation_id", "deleteRelation", []string{"expected_revision"}},
	{"POST", "/v1/worldview/relations/:relation_id/restore", "restoreRelation", []string{}},
	{"GET", "/v1/worldview/novels/:novel_id/graph", "graph", []string{"center_id", "depth"}},
	{"GET", "/v1/worldview/novels/:novel_id/world-revision", "getWorldRevision", []string{}},
	{"GET", "/v1/worldview/novels/:novel_id/facts", "listFacts", []string{"limit", "cursor", "subject_id", "status"}},
	{"GET", "/v1/worldview/novels/:novel_id/fact-events", "listFactEvents", []string{"limit", "cursor", "subject_id"}},
	{"POST", "/v1/worldview/novels/:novel_id/facts/check", "checkFacts", []string{}},
	{"POST", "/v1/worldview/facts/:fact_id/retract", "retractFact", []string{}},
	{"POST", "/v1/worldview/facts/:fact_id/supersede", "supersedeFact", []string{}},
}
