package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/assistant"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/knowledge"
	"github.com/chiotron/ai-control-plane/internal/provider"
)

var errProvider = errors.New("provider unavailable")

type stubRetriever struct {
	rounds  [][]knowledge.Hit
	queries []string
	err     error
}

func (s *stubRetriever) Search(_ context.Context, query string, _ []float32,
	_ knowledge.Access, _ int) ([]knowledge.Hit, error) {
	s.queries = append(s.queries, query)
	if s.err != nil {
		return nil, s.err
	}
	index := len(s.queries) - 1
	if index >= len(s.rounds) {
		return nil, nil
	}
	return s.rounds[index], nil
}

type stubEmbedder struct{ err error }

func (s stubEmbedder) Embed(_ context.Context, inputs []string) ([][]float32, error) {
	if s.err != nil {
		return nil, s.err
	}
	vectors := make([][]float32, len(inputs))
	for i := range inputs {
		vectors[i] = []float32{1, 0, 0}
	}
	return vectors, nil
}

type stubCompleter struct {
	answer   string
	err      error
	messages []provider.Message
	// route is what Resolve reports. The zero value has no ceiling, so tests
	// that are not about egress are unaffected by it.
	route provider.Route
}

func (s *stubCompleter) Resolve(string) (provider.LLM, provider.Route, error) {
	return nil, s.route, nil
}

func (s *stubCompleter) Chat(_ context.Context, _ string, req provider.ChatRequest) (provider.ChatResponse, provider.Route, error) {
	s.messages = req.Messages
	if s.err != nil {
		return provider.ChatResponse{}, provider.Route{}, s.err
	}
	return provider.ChatResponse{
			Model: "test-model", Content: s.answer, FinishReason: "stop",
			Usage: provider.Usage{PromptTokens: 20, CompletionTokens: 5, TotalTokens: 25},
		},
		provider.Route{Logical: "default", Provider: "stub", Model: "test-model"}, nil
}

func testOrchestrator(t *testing.T, retriever *stubRetriever, completer *stubCompleter, minScore float64) *Orchestrator {
	t.Helper()
	classes, err := knowledge.NewPolicy([]string{"public", "internal"})
	if err != nil {
		t.Fatalf("NewPolicy() returned error: %v", err)
	}
	policy, err := NewPolicy(3, 5, minScore, 0.15)
	if err != nil {
		t.Fatalf("NewPolicy() returned error: %v", err)
	}
	return &Orchestrator{
		Retriever: retriever, Embedder: stubEmbedder{}, Completer: completer,
		Policy: policy, Classes: classes,
	}
}

func testRequest(mode, question string) Request {
	return Request{
		Caller: auth.Identity{KeyID: "k", MaxClassification: "internal", Scopes: auth.KnownScopes},
		Assistant: assistant.Assistant{
			Slug: "general", LogicalModel: "default",
			Instructions: "Be concise.", Retrieval: mode, MaxSteps: 3,
		},
		Question: question,
	}
}

func TestAnswerGroundsAndMarksCitations(t *testing.T) {
	retriever := &stubRetriever{rounds: [][]knowledge.Hit{{
		hit(1, "doc-a", "Runbook", 0, 0.033),
	}}}
	completer := &stubCompleter{answer: "Mint a replacement, then revoke the old key [1]."}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	answer, err := orchestrator.Answer(context.Background(), testRequest(RetrievalAuto, "how do I rotate a key"))
	if err != nil {
		t.Fatalf("Answer() returned error: %v", err)
	}

	if !answer.Grounded || len(answer.UsedCited) != 1 {
		t.Errorf("answer is not grounded: grounded=%v cited=%v", answer.Grounded, answer.UsedCited)
	}
	if len(answer.Citations) != 1 || !answer.Citations[0].Used {
		t.Errorf("citations = %+v, want one marked used", answer.Citations)
	}
	// The passages must actually reach the model, and the instruction to cite
	// them with them.
	system := answer.Steps[0]
	if system.Kind != StepPlan {
		t.Errorf("first step = %q, want the plan", system.Kind)
	}
	if !strings.Contains(completer.messages[0].Content, "square brackets") {
		t.Error("the synthesis prompt does not ask for citations")
	}
	if !strings.Contains(completer.messages[1].Content, "[1] Runbook") {
		t.Errorf("the passages were not offered to the model:\n%s", completer.messages[1].Content)
	}
}

// A model handed passages can ignore them. The answer has to say so rather than
// claim to be grounded.
func TestAnswerReportsAnUngroundedAnswer(t *testing.T) {
	retriever := &stubRetriever{rounds: [][]knowledge.Hit{{hit(1, "doc-a", "Runbook", 0, 0.033)}}}
	completer := &stubCompleter{answer: "Keys should be rotated regularly."}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	answer, err := orchestrator.Answer(context.Background(), testRequest(RetrievalAuto, "how do I rotate a key"))
	if err != nil {
		t.Fatalf("Answer() returned error: %v", err)
	}
	if answer.Grounded {
		t.Error("an answer with no citation markers was reported as grounded")
	}
	if len(answer.Citations) != 1 {
		t.Errorf("citations = %d, want the offered passage still reported", len(answer.Citations))
	}
}

// Weak evidence is worse than none: passing low-scoring passages invites the
// model to answer from them anyway.
func TestAnswerDiscardsWeakEvidence(t *testing.T) {
	retriever := &stubRetriever{rounds: [][]knowledge.Hit{
		{hit(1, "doc-a", "Runbook", 0, 0.0001)},
		{hit(1, "doc-a", "Runbook", 0, 0.0001)},
	}}
	completer := &stubCompleter{answer: "I do not know."}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	answer, err := orchestrator.Answer(context.Background(), testRequest(RetrievalAuto, "how do I rotate a key"))
	if err != nil {
		t.Fatalf("Answer() returned error: %v", err)
	}
	if len(answer.Citations) != 0 {
		t.Errorf("citations = %+v, want none after discarding weak evidence", answer.Citations)
	}
	if strings.Contains(completer.messages[len(completer.messages)-1].Content, "Passages:") {
		t.Error("discarded passages still reached the model")
	}
	if !hasStep(answer.Steps, StepRetrieve, OutcomeSkipped) {
		t.Errorf("the discard was not recorded in the trace: %+v", answer.Steps)
	}
}

// A first round that is not good enough triggers one expanded follow-up, anchored
// on the strongest document.
func TestAnswerExpandsTheQueryOnWeakFirstRound(t *testing.T) {
	retriever := &stubRetriever{rounds: [][]knowledge.Hit{
		{hit(1, "doc-a", "Key rotation runbook", 0, 0.001)},
		{hit(2, "doc-a", "Key rotation runbook", 1, 0.033)},
	}}
	completer := &stubCompleter{answer: "Revoke the old key [2]."}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	answer, err := orchestrator.Answer(context.Background(), testRequest(RetrievalAuto, "how do I rotate"))
	if err != nil {
		t.Fatalf("Answer() returned error: %v", err)
	}
	if len(retriever.queries) != 2 {
		t.Fatalf("ran %d searches, want 2: %v", len(retriever.queries), retriever.queries)
	}
	if !strings.HasPrefix(retriever.queries[1], "Key rotation runbook") {
		t.Errorf("the follow-up query was not anchored on the best document: %q", retriever.queries[1])
	}
	if !answer.Grounded {
		t.Error("the expanded round did not produce a grounded answer")
	}
}

func TestAnswerSkipsRetrievalForShortUtterances(t *testing.T) {
	retriever := &stubRetriever{}
	completer := &stubCompleter{answer: "Hello."}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	answer, err := orchestrator.Answer(context.Background(), testRequest(RetrievalAuto, "hi"))
	if err != nil {
		t.Fatalf("Answer() returned error: %v", err)
	}
	if len(retriever.queries) != 0 {
		t.Errorf("ran %d searches for a greeting, want none", len(retriever.queries))
	}
	if !hasStep(answer.Steps, StepRetrieve, OutcomeSkipped) {
		t.Errorf("the skip was not recorded: %+v", answer.Steps)
	}
}

// Losing the corpus degrades an answer to ungrounded. Denying the caller an
// answer entirely would make the knowledge platform a single point of failure
// for chat.
func TestAnswerSurvivesRetrievalFailure(t *testing.T) {
	retriever := &stubRetriever{err: errProvider}
	completer := &stubCompleter{answer: "I could not consult the knowledge base."}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	answer, err := orchestrator.Answer(context.Background(), testRequest(RetrievalAuto, "how do I rotate a key"))
	if err != nil {
		t.Fatalf("Answer() returned error: %v", err)
	}
	if answer.Content == "" {
		t.Error("no answer was produced despite the model being available")
	}
	if !hasStep(answer.Steps, StepRetrieve, OutcomeFailure) {
		t.Errorf("the retrieval failure was not recorded: %+v", answer.Steps)
	}
}

// An assistant configured to ground every answer must be told to say so when
// there is nothing to ground it in, rather than answering from the model alone.
func TestAnswerInstructsRefusalWhenAlwaysGroundedFindsNothing(t *testing.T) {
	retriever := &stubRetriever{}
	completer := &stubCompleter{answer: "The knowledge base does not cover this."}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	if _, err := orchestrator.Answer(context.Background(), testRequest(RetrievalAlways, "what is the policy")); err != nil {
		t.Fatalf("Answer() returned error: %v", err)
	}
	if !strings.Contains(completer.messages[0].Content, "does not cover") {
		t.Errorf("the prompt does not instruct a refusal:\n%s", completer.messages[0].Content)
	}
}

func TestAnswerFlagsConflictingSources(t *testing.T) {
	retriever := &stubRetriever{rounds: [][]knowledge.Hit{{
		hitWithVector(1, "doc-a", "Policy 2025", 0, 0.0330, 0.81),
		hitWithVector(2, "doc-b", "Policy 2026", 0, 0.0325, 0.78),
	}}}
	completer := &stubCompleter{answer: "The two policies differ [1][2]."}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	answer, err := orchestrator.Answer(context.Background(), testRequest(RetrievalAuto, "what is the retention policy"))
	if err != nil {
		t.Fatalf("Answer() returned error: %v", err)
	}
	if !answer.Conflicted {
		t.Error("competing sources were not flagged")
	}
	if !strings.Contains(completer.messages[0].Content, "disagree") {
		t.Errorf("the prompt does not ask for both positions:\n%s", completer.messages[0].Content)
	}
}

// A model outage is a real failure: there is no answer to give.
func TestAnswerFailsWhenTheModelFails(t *testing.T) {
	retriever := &stubRetriever{}
	completer := &stubCompleter{err: errProvider}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	answer, err := orchestrator.Answer(context.Background(), testRequest(RetrievalOff, "anything at all"))
	if !errors.Is(err, errProvider) {
		t.Fatalf("error = %v, want the provider failure", err)
	}
	// The trace is still returned so the failure can be recorded.
	if len(answer.Steps) == 0 {
		t.Error("no trace was returned for a failed run")
	}
}

// The plan record and the synthesis must not consume investigation budget: a
// three-step run should get three real steps of looking.
func TestInvestigationStepsCountsOnlyWork(t *testing.T) {
	steps := []Step{
		{Kind: StepPlan, Outcome: OutcomeSuccess},
		{Kind: StepRetrieve, Outcome: OutcomeSuccess},
		{Kind: StepRetrieve, Outcome: OutcomeSkipped},
		{Kind: StepTool, Outcome: OutcomeSuccess},
		{Kind: StepTool, Outcome: OutcomeFailure},
		{Kind: StepSynthesise, Outcome: OutcomeSuccess},
	}

	// One retrieval, one tool success and one tool failure are work; the plan, the
	// skip and the synthesis are not.
	if got := investigationSteps(steps); got != 3 {
		t.Errorf("investigationSteps() = %d, want 3", got)
	}
}

// A retrieval that finished in one round must leave the rest of the budget for
// tools, rather than the plan record eating it.
func TestAnswerSpendsBudgetOnToolsAfterASingleRetrieval(t *testing.T) {
	retriever := &stubRetriever{rounds: [][]knowledge.Hit{{hit(1, "doc-a", "Runbook", 0, 0.033)}}}
	completer := &stubCompleter{answer: "See [1]."}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	answer, err := orchestrator.Answer(context.Background(), testRequest(RetrievalAuto, "how do I rotate a key"))
	if err != nil {
		t.Fatalf("Answer() returned error: %v", err)
	}
	if investigationSteps(answer.Steps) != 1 {
		t.Fatalf("investigation steps = %d, want 1 retrieval", investigationSteps(answer.Steps))
	}
	// Two of the three steps are still unspent, which is what leaves room for the
	// caller's tools.
	if remaining := orchestrator.Policy.MaxSteps - investigationSteps(answer.Steps); remaining != 2 {
		t.Errorf("remaining budget = %d, want 2", remaining)
	}
}

func hasStep(steps []Step, kind, outcome string) bool {
	for _, step := range steps {
		if step.Kind == kind && step.Outcome == outcome {
			return true
		}
	}
	return false
}

// The egress ceiling is the guard that makes a cloud provider safe to configure
// at all: retrieval filters by what the caller may read, and this asks the
// different question of what the provider may be told.

func TestAnswerRefusesContextAboveTheProviderCeiling(t *testing.T) {
	retriever := &stubRetriever{rounds: [][]knowledge.Hit{{
		classifiedHit(1, "doc-a", "Board minutes", 0.9, "internal"),
	}}}
	completer := &stubCompleter{
		answer: "should never be produced",
		route:  provider.Route{Provider: "openai", MaxClassification: "public"},
	}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	_, err := orchestrator.Answer(context.Background(), testRequest(RetrievalAuto, "what did the board decide"))
	if !errors.Is(err, ErrEgressRefused) {
		t.Fatalf("Answer() error = %v, want ErrEgressRefused", err)
	}
	// The point of the guard is that nothing was sent, not that the answer was
	// thrown away after the fact.
	if len(completer.messages) != 0 {
		t.Fatalf("internal context reached the provider anyway: %d messages", len(completer.messages))
	}
}

func TestAnswerProceedsWhenTheContextIsWithinTheCeiling(t *testing.T) {
	// An internal-cleared caller whose question retrieves only public passages
	// must still be answered by a public-only provider.
	retriever := &stubRetriever{rounds: [][]knowledge.Hit{{
		classifiedHit(1, "doc-a", "Public handbook", 0.9, "public"),
	}}}
	completer := &stubCompleter{
		answer: "the handbook says this [1]",
		route:  provider.Route{Provider: "openai", MaxClassification: "public"},
	}
	orchestrator := testOrchestrator(t, retriever, completer, 0.016)

	answer, err := orchestrator.Answer(context.Background(), testRequest(RetrievalAuto, "what does the handbook say"))
	if err != nil {
		t.Fatalf("Answer() returned error: %v", err)
	}
	if answer.Content == "" {
		t.Fatal("Answer() produced no content")
	}
	if len(completer.messages) == 0 {
		t.Fatal("a permitted call never reached the provider")
	}
}
