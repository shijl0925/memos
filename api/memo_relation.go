package api

// MemoRelationType is the type of relationship between two memos.
type MemoRelationType string

const (
	// MemoRelationReference means the memo references another memo.
	MemoRelationReference MemoRelationType = "REFERENCE"
	// MemoRelationAdditional means the memo provides additional context for another memo.
	MemoRelationAdditional MemoRelationType = "ADDITIONAL"
)

// MemoRelation is the API type for a memo-to-memo relationship.
type MemoRelation struct {
	MemoID        int              `json:"memoId"`
	RelatedMemoID int              `json:"relatedMemoId"`
	Type          MemoRelationType `json:"type"`
}

// MemoRelationUpsert is the payload used when creating or updating a memo relation.
type MemoRelationUpsert struct {
	RelatedMemoID int              `json:"relatedMemoId"`
	Type          MemoRelationType `json:"type"`
}
