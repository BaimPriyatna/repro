package entity_test

import (
	"testing"

	"github.com/BaimPriyatna/repro/src/core/entity"
)

func TestEntity_IsVersioned(t *testing.T) {
	t.Run("versioned", func(t *testing.T) {
		e := &entity.Entity{
			ID:      "pkg-react",
			Kind:    entity.KindPackage,
			Name:    "react",
			Version: "18.2.0",
		}
		if !e.IsVersioned() {
			t.Error("IsVersioned() should be true when Version is set")
		}
	})

	t.Run("unversioned", func(t *testing.T) {
		e := &entity.Entity{
			ID:   "file-hosts",
			Kind: entity.KindFile,
			Name: "/etc/hosts",
		}
		if e.IsVersioned() {
			t.Error("IsVersioned() should be false when Version is empty")
		}
	})
}

func TestEntityKindConstants_NonEmpty(t *testing.T) {
	kinds := []entity.Kind{
		entity.KindPackage,
		entity.KindFile,
		entity.KindService,
		entity.KindRuntime,
		entity.KindConfig,
		entity.KindTool,
		entity.KindModule,
		entity.KindUnknown,
	}
	for _, k := range kinds {
		if k == "" {
			t.Errorf("Entity Kind constant must not be empty string")
		}
	}
}

func TestEntity_Attributes_Preserved(t *testing.T) {
	e := &entity.Entity{
		ID:   "pkg-go",
		Kind: entity.KindRuntime,
		Name: "go",
		Attributes: map[string]any{
			"arch": "amd64",
			"os":   "linux",
		},
	}
	if e.Attributes["arch"] != "amd64" {
		t.Errorf("Attributes[arch] = %v; want amd64", e.Attributes["arch"])
	}
}
