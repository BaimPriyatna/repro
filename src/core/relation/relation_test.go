package relation_test

import (
	"testing"

	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/relation"
)

func TestRelation_Involves_FromID(t *testing.T) {
	r := &relation.Relation{
		ID:     "rel-001",
		Kind:   relation.KindDependsOn,
		FromID: entity.ID("pkg-A"),
		ToID:   entity.ID("pkg-B"),
	}
	if !r.Involves(entity.ID("pkg-A")) {
		t.Error("Involves should return true for the FromID")
	}
}

func TestRelation_Involves_ToID(t *testing.T) {
	r := &relation.Relation{
		ID:     "rel-002",
		Kind:   relation.KindImports,
		FromID: entity.ID("mod-x"),
		ToID:   entity.ID("mod-y"),
	}
	if !r.Involves(entity.ID("mod-y")) {
		t.Error("Involves should return true for the ToID")
	}
}

func TestRelation_Involves_Unrelated(t *testing.T) {
	r := &relation.Relation{
		ID:     "rel-003",
		Kind:   relation.KindUses,
		FromID: entity.ID("svc-api"),
		ToID:   entity.ID("svc-db"),
	}
	if r.Involves(entity.ID("svc-cache")) {
		t.Error("Involves should return false for an unrelated entity")
	}
}

func TestRelationKindConstants_NonEmpty(t *testing.T) {
	kinds := []relation.Kind{
		relation.KindDependsOn,
		relation.KindImports,
		relation.KindUses,
		relation.KindGeneratedBy,
		relation.KindConfiguredBy,
		relation.KindOwnedBy,
		relation.KindAffects,
	}
	for _, k := range kinds {
		if k == "" {
			t.Errorf("Relation Kind constant must not be empty string")
		}
	}
}
