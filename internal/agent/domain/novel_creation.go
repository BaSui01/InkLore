package domain

import (
	"encoding/json"
	"time"
	"unicode/utf8"
)

const (
	NovelGuideTaskKey           = "novel_architect"
	NovelGuideDefinitionKey     = "novel_architect"
	NovelGuideAskUserTool       = "ask_user"
	NovelGuideProposeNovelTool  = "propose_novel"
	NovelGuideMaxTurns          = 8
	NovelGuideMaxIdeaCodepoints = 2000
	NovelGuideMaxAnswerCodepts  = 2000
)

type NovelGuideStatus string

const (
	NovelGuideProcessing  NovelGuideStatus = "processing"
	NovelGuideWaitingUser NovelGuideStatus = "waiting_user"
	NovelGuideProposed    NovelGuideStatus = "proposed"
	NovelGuideFailed      NovelGuideStatus = "failed"
	NovelGuideCompleted   NovelGuideStatus = "completed"
)

type NovelGuideQuestion struct {
	Text        string   `json:"text"`
	Suggestions []string `json:"suggestions"`
}

type NovelGuideTurn struct {
	Question    string   `json:"question"`
	Suggestions []string `json:"suggestions"`
	Answer      string   `json:"answer"`
}

type NovelGuideProposal struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	Summary     string `json:"summary"`
	TargetWords int64  `json:"target_words"`
}

type NovelGuideSession struct {
	ID                  string              `json:"session_id"`
	TenantID            string              `json:"-"`
	OwnerUserID         string              `json:"-"`
	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
	Revision            int64               `json:"revision"`
	Status              NovelGuideStatus    `json:"status"`
	InitialIdea         string              `json:"initial_idea"`
	Turns               []NovelGuideTurn    `json:"turns"`
	CurrentQuestion     *NovelGuideQuestion `json:"current_question,omitempty"`
	Proposal            *NovelGuideProposal `json:"proposal,omitempty"`
	CreatedNovelID      string              `json:"created_novel_id,omitempty"`
	DefinitionVersionID string              `json:"-"`
	SystemInstruction   string              `json:"-"`
	PolicyVersionID     string              `json:"-"`
	ModelName           string              `json:"-"`
	MaxOutputTokens     int                 `json:"-"`
	Temperature         float64             `json:"-"`
	StartKey            string              `json:"-"`
	StartRequestHash    string              `json:"-"`
	StartResponse       json.RawMessage     `json:"-"`
	ErrorCode           string              `json:"error_code,omitempty"`
	ExpiresAt           time.Time           `json:"expires_at"`
}

type NovelGuideCommand struct {
	ID             string
	TenantID       string
	OwnerUserID    string
	SessionID      string
	CommandType    string
	IdempotencyKey string
	RequestHash    string
	Status         string
	Response       json.RawMessage
	ErrorCode      string
}

type NovelGuideStartInput struct {
	InitialIdea string `json:"initial_idea"`
}

type NovelGuideAnswerInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Answer           string `json:"answer"`
}

type NovelGuideCompleteInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	NovelID          string `json:"novel_id"`
}

type NovelGuidePolicySnapshot struct {
	VersionID       string
	ModelName       string
	MaxOutputTokens int
	Temperature     float64
}

func ValidateNovelGuideIdea(idea string) bool {
	return utf8.ValidString(idea) && utf8.RuneCountInString(idea) <= NovelGuideMaxIdeaCodepoints
}

func ValidateNovelGuideAnswer(answer string) bool {
	return utf8.ValidString(answer) && utf8.RuneCountInString(answer) >= 1 && utf8.RuneCountInString(answer) <= NovelGuideMaxAnswerCodepts
}

func ValidateNovelGuideQuestion(question NovelGuideQuestion) bool {
	if !utf8.ValidString(question.Text) || utf8.RuneCountInString(question.Text) < 1 || utf8.RuneCountInString(question.Text) > 500 {
		return false
	}
	if len(question.Suggestions) < 2 || len(question.Suggestions) > 5 {
		return false
	}
	for _, suggestion := range question.Suggestions {
		if !utf8.ValidString(suggestion) || utf8.RuneCountInString(suggestion) < 1 || utf8.RuneCountInString(suggestion) > 120 {
			return false
		}
	}
	return true
}

func ValidateNovelGuideProposal(proposal NovelGuideProposal) bool {
	return utf8.ValidString(proposal.Title) && utf8.RuneCountInString(proposal.Title) >= 1 && utf8.RuneCountInString(proposal.Title) <= 120 &&
		utf8.ValidString(proposal.Category) && utf8.RuneCountInString(proposal.Category) <= 60 &&
		utf8.ValidString(proposal.Summary) && utf8.RuneCountInString(proposal.Summary) <= 2000 &&
		proposal.TargetWords >= 0 && proposal.TargetWords <= 100000000
}
