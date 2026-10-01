// Package ghostfile tracks file origin, usage, duplication, and lifecycle.
//
// Lifecycle stages: Created → Modified → Used → Copied → Unused → Deleted.
package ghostfile

import (
	"fmt"
	"time"

	"github.com/BaimPriyatna/repro/src/analysis"
	"github.com/BaimPriyatna/repro/src/analysis/history"
	coreanalysis "github.com/BaimPriyatna/repro/src/core/analysis"
	"github.com/BaimPriyatna/repro/src/core/entity"
	"github.com/BaimPriyatna/repro/src/core/snapshot"
	"github.com/BaimPriyatna/repro/src/graph"
	"github.com/BaimPriyatna/repro/src/graph/repairmap"
)

const (
	// AnalyzerName is the unique identifier for the GhostFile analyzer.
	AnalyzerName = "ghostfile"

	// AnalyzerVersion is the version of this analyzer implementation.
	AnalyzerVersion = "1.0.0"

	// Finding Types.
	FindingTypeUnused                = "ghost_file_unused"
	FindingTypeCopied                = "ghost_file_copied"
	FindingTypeDeletedWithReferences = "ghost_file_deleted_with_references"
	FindingTypeLifecycle             = "ghost_file_lifecycle"
)

// LifecycleState represents the lifecycle status of a file.
type LifecycleState string

const (
	StateCreated  LifecycleState = "created"
	StateModified LifecycleState = "modified"
	StateUsed     LifecycleState = "used"
	StateCopied   LifecycleState = "copied"
	StateUnused   LifecycleState = "unused"
	StateDeleted  LifecycleState = "deleted"
)

// FileInfo holds metadata about a file observed in snapshots or graph.
type FileInfo struct {
	Path     string
	Hash     string
	Snapshot snapshot.ID
	EntityID entity.ID
}

// GhostFile analyzes file lifecycle by combining Snapshot History and Usage Graph.
type GhostFile struct {
	repairMap *repairmap.RepairMap
	chain     *analysis.HistoryChain
}

// New creates a new GhostFile analyzer with a RepairMap and a pre-traversed HistoryChain.
func New(rm *repairmap.RepairMap, chain *analysis.HistoryChain) (*GhostFile, error) {
	if rm == nil {
		return nil, fmt.Errorf("ghostfile: repair map must not be nil")
	}
	if chain == nil || chain.IsEmpty() {
		return nil, fmt.Errorf("ghostfile: history chain must not be nil or empty: %w", analysis.ErrInvalidInput)
	}

	return &GhostFile{
		repairMap: rm,
		chain:     chain,
	}, nil
}

// NewWithTraverser creates a new GhostFile analyzer using a history traverser.
func NewWithTraverser(rm *repairmap.RepairMap, startSnapshot *snapshot.Snapshot, traverser *history.Traverser, maxDepth int) (*GhostFile, error) {
	if rm == nil || startSnapshot == nil || traverser == nil {
		return nil, analysis.ErrInvalidInput
	}
	if maxDepth <= 0 {
		maxDepth = 20
	}

	chain, err := traverser.Traverse(analysis.HistoryQuery{
		StartID:      startSnapshot.ID,
		IncludeStart: true,
		MaxDepth:     maxDepth,
	})
	if err != nil {
		return nil, fmt.Errorf("ghostfile: history traversal failed: %w", err)
	}

	return New(rm, chain)
}

// Name returns the analyzer identifier.
func (gf *GhostFile) Name() string {
	return AnalyzerName
}

// Version returns the analyzer version.
func (gf *GhostFile) Version() string {
	return AnalyzerVersion
}

// Analyze performs dual-branch analysis combining Snapshot History and RepairMap.
func (gf *GhostFile) Analyze() (*coreanalysis.AnalysisResult, error) {
	newestSnap := gf.chain.Newest()
	resultID := coreanalysis.ResultID(fmt.Sprintf("ghostfile-%s-%d", newestSnap.ID, time.Now().UTC().UnixNano()))

	result := &coreanalysis.AnalysisResult{
		ID:               resultID,
		Analyzer:         AnalyzerName,
		Version:          AnalyzerVersion,
		Timestamp:        time.Now().UTC(),
		Status:           coreanalysis.StatusOK,
		Findings:         make([]coreanalysis.Finding, 0),
		Evidence:         make([]coreanalysis.DiagnosticEvidence, 0),
		RelatedSnapshots: make([]snapshot.ID, len(gf.chain.Snapshots)),
	}

	for i, snap := range gf.chain.Snapshots {
		result.RelatedSnapshots[i] = snap.ID
	}

	// 1. Reconstruct historical file appearances and modifications
	// Chain is ordered newest (index 0) to oldest (index N-1).
	// We reverse to analyze chronologically (oldest -> newest).
	chronological := make([]*snapshot.Snapshot, len(gf.chain.Snapshots))
	for i, snap := range gf.chain.Snapshots {
		chronological[len(gf.chain.Snapshots)-1-i] = snap
	}

	// Track files per path: path -> latest hash & appearance history
	fileHistory := make(map[string][]FileInfo)
	hashToPaths := make(map[string]map[string]bool)

	for _, snap := range chronological {
		snapFiles := extractFilesFromSnapshot(snap)
		for path, hash := range snapFiles {
			fileHistory[path] = append(fileHistory[path], FileInfo{
				Path:     path,
				Hash:     hash,
				Snapshot: snap.ID,
			})

			if hash != "" {
				if hashToPaths[hash] == nil {
					hashToPaths[hash] = make(map[string]bool)
				}
				hashToPaths[hash][path] = true
			}
		}
	}

	// 2. Query Usage Graph for file entities
	fileEntities, err := gf.repairMap.ListEntities(graph.Filter{
		EntityKind: entity.KindFile,
	})
	if err != nil {
		return nil, fmt.Errorf("ghostfile: failed to list file entities from repair map: %w", err)
	}

	// Map file entities by path / ID
	graphFileMap := make(map[string]*entity.Entity)
	for _, fe := range fileEntities {
		path := fe.Name
		if path == "" {
			path = string(fe.ID)
		}
		graphFileMap[path] = fe
		graphFileMap[string(fe.ID)] = fe
	}

	// Latest snapshot files
	latestFiles := extractFilesFromSnapshot(newestSnap)

	// 3. Evaluate each file tracked in graph or history
	allTrackedPaths := make(map[string]bool)
	for path := range fileHistory {
		allTrackedPaths[path] = true
	}
	for path := range latestFiles {
		allTrackedPaths[path] = true
	}
	for path := range graphFileMap {
		allTrackedPaths[path] = true
	}

	for path := range allTrackedPaths {
		historyEntries := fileHistory[path]
		inLatestSnapshot := false
		if _, ok := latestFiles[path]; ok {
			inLatestSnapshot = true
		}

		fe := graphFileMap[path]
		var dependents []*entity.Entity
		if fe != nil {
			dependents, _ = gf.repairMap.DirectDependents(fe.ID, "")
		}

		isReferencedInGraph := len(dependents) > 0

		// Check Case A: Deleted in latest snapshot, but still referenced in RepairMap
		if !inLatestSnapshot && len(historyEntries) > 0 && isReferencedInGraph {
			findingID := coreanalysis.FindingID(fmt.Sprintf("ghost-deleted-ref-%s", path))
			evidence := []coreanalysis.DiagnosticEvidence{
				{
					Kind:     coreanalysis.EvidenceKindFile,
					SourceID: path,
					Field:    "lifecycle_state",
					Value:    string(StateDeleted),
				},
				{
					Kind:     coreanalysis.EvidenceKindSnapshot,
					SourceID: string(newestSnap.ID),
					Field:    "missing_in_snapshot",
					Value:    path,
				},
			}
			if fe != nil {
				evidence = append(evidence, coreanalysis.DiagnosticEvidence{
					Kind:     coreanalysis.EvidenceKindRelation,
					SourceID: string(fe.ID),
					Field:    "incoming_references_count",
					Value:    len(dependents),
				})
			}

			result.Findings = append(result.Findings, coreanalysis.Finding{
				ID:          findingID,
				Severity:    coreanalysis.SeverityHigh,
				Type:        FindingTypeDeletedWithReferences,
				Subject:     entity.ID(path),
				Description: fmt.Sprintf("File '%s' was deleted in history but is still referenced by %d component(s) in the usage graph", path, len(dependents)),
				Confidence:  coreanalysis.ConfidenceConfirmed,
				Evidence:    evidence,
			})
			continue
		}

		// Check Case B: Present in snapshot/graph, but unused (zero incoming references in RepairMap)
		if inLatestSnapshot && !isReferencedInGraph {
			findingID := coreanalysis.FindingID(fmt.Sprintf("ghost-unused-%s", path))
			evidence := []coreanalysis.DiagnosticEvidence{
				{
					Kind:     coreanalysis.EvidenceKindFile,
					SourceID: path,
					Field:    "lifecycle_state",
					Value:    string(StateUnused),
				},
				{
					Kind:     coreanalysis.EvidenceKindSnapshot,
					SourceID: string(newestSnap.ID),
					Field:    "snapshot.id",
					Value:    string(newestSnap.ID),
				},
			}
			if fe != nil {
				evidence = append(evidence, coreanalysis.DiagnosticEvidence{
					Kind:     coreanalysis.EvidenceKindRelation,
					SourceID: string(fe.ID),
					Field:    "incoming_references_count",
					Value:    0,
				})
			}

			result.Findings = append(result.Findings, coreanalysis.Finding{
				ID:          findingID,
				Severity:    coreanalysis.SeverityLow,
				Type:        FindingTypeUnused,
				Subject:     entity.ID(path),
				Description: fmt.Sprintf("File '%s' exists in the environment but is unused in the usage graph (ghost file)", path),
				Confidence:  coreanalysis.ConfidenceHigh,
				Evidence:    evidence,
			})
		}

		// Check Case C: File is duplicated / copied
		if len(historyEntries) > 0 {
			latestHash := historyEntries[len(historyEntries)-1].Hash
			if latestHash != "" && len(hashToPaths[latestHash]) > 1 {
				copiedPaths := make([]string, 0, len(hashToPaths[latestHash]))
				for cp := range hashToPaths[latestHash] {
					if cp != path {
						copiedPaths = append(copiedPaths, cp)
					}
				}
				if len(copiedPaths) > 0 {
					findingID := coreanalysis.FindingID(fmt.Sprintf("ghost-copied-%s", path))
					result.Findings = append(result.Findings, coreanalysis.Finding{
						ID:          findingID,
						Severity:    coreanalysis.SeverityInfo,
						Type:        FindingTypeCopied,
						Subject:     entity.ID(path),
						Description: fmt.Sprintf("File '%s' shares identical content hash '%s' with %v", path, latestHash, copiedPaths),
						Confidence:  coreanalysis.ConfidenceHigh,
						Evidence: []coreanalysis.DiagnosticEvidence{
							{
								Kind:     coreanalysis.EvidenceKindFile,
								SourceID: path,
								Field:    "content_hash",
								Value:    latestHash,
							},
							{
								Kind:     coreanalysis.EvidenceKindSnapshot,
								SourceID: string(newestSnap.ID),
								Field:    "duplicate_files",
								Value:    copiedPaths,
							},
						},
					})
				}
			}
		}
	}

	if len(result.Findings) > 0 {
		result.Status = coreanalysis.StatusFindings
	}

	return result, nil
}

// extractFilesFromSnapshot extracts a map of filepath -> hash from snapshot.Data.
func extractFilesFromSnapshot(s *snapshot.Snapshot) map[string]string {
	files := make(map[string]string)
	if s == nil || s.Data == nil {
		return files
	}

	// Check "files" or "filesystem" collector data
	if raw, ok := s.Data["files"]; ok {
		if fileMap, ok := raw.(map[string]any); ok {
			for path, v := range fileMap {
				if hashStr, ok := v.(string); ok {
					files[path] = hashStr
				} else if metaMap, ok := v.(map[string]any); ok {
					if h, ok := metaMap["hash"].(string); ok {
						files[path] = h
					} else {
						files[path] = ""
					}
				} else {
					files[path] = ""
				}
			}
		} else if fileSlice, ok := raw.([]string); ok {
			for _, path := range fileSlice {
				files[path] = ""
			}
		}
	}

	if raw, ok := s.Data["filesystem"]; ok {
		if fileMap, ok := raw.(map[string]any); ok {
			for path, v := range fileMap {
				if hashStr, ok := v.(string); ok {
					files[path] = hashStr
				}
			}
		}
	}

	return files
}
