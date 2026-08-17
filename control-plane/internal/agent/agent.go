package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chiotron/ai-control-plane/internal/assistant"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/knowledge"
	"github.com/chiotron/ai-control-plane/internal/provider"
	"github.com/chiotron/ai-control-plane/internal/tool"
)

// Step kinds recorded in a run trace.
const (
	StepPlan       = "plan"
	StepRetrieve   = "retrieve"
	StepTool       = "tool"
	StepSynthesise = "synthesise"
)

const (
	OutcomeSuccess = "success"
	OutcomeSkipped = "skipped"
	OutcomeFailure = "failure"
)

// Step is one recorded action, so an answer can be explained after the fact.
type Step struct {
	Ordinal int            `json:"ordinal"`
	Kind    string         `json:"kind"`
	Summary string         `json:"summary"`
	Detail  map[string]any `json:"detail,omitempty"`
	Outcome string         `json:"outcome"`
	Latency time.Duration  `json:"-"`
	Millis  int64          `json:"latencyMs"`
}

// Answer is a completed run.
type Answer struct {
	RunID      string          `json:"runId"`
	Content    string          `json:"content"`
	Assistant  string          `json:"assistant"`
	Model      string          `json:"model"`
	Provider   string          `json:"provider"`
	Citations  []Citation      `json:"citations"`
	UsedCited  []int           `json:"citedIndices"`
	Grounded   bool            `json:"grounded"`
	Conflicted bool            `json:"conflicted"`
	Usage      provider.Usage  `json:"usage"`
	LatencyMs  int64           `json:"latencyMs"`
	Steps      []Step          `json:"steps"`
	ToolsUsed  []string        `json:"toolsUsed,omitempty"`
	Retrieval  string          `json:"retrievalMode"`
	Hits       []knowledge.Hit `json:"-"`
}

// Request is one question to answer.
type Request struct {
	Caller         auth.Identity
	Assistant      assistant.Assistant
	Question       string
	ConversationID string
	// Tools the caller explicitly asked for, beyond retrieval. Each is still
	// authorized by the registry.
	Tools []string
}

// Retriever is the corpus search the orchestrator grounds answers in.
type Retriever interface {
	Search(ctx context.Context, query string, embedding []float32, access knowledge.Access, limit int) ([]knowledge.Hit, error)
}

// Embedder vectorises a query.
type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

// Completer is the model call. It takes a logical model id, so the orchestrator
// never names a provider.
type Completer interface {
	Chat(ctx context.Context, logical string, req provider.ChatRequest) (provider.ChatResponse, provider.Route, error)
}

// Orchestrator plans and executes a grounded answer.
type Orchestrator struct {
	Retriever Retriever
	Embedder  Embedder
	Completer Completer
	Tools     *tool.Registry
	Policy    Policy
	Classes   knowledge.Policy
}

// Answer runs the plan: retrieve, expand if weak, call requested tools, then
// synthesise with citations.
//
// A retrieval failure never fails the run. Losing the corpus or the embedding
// provider degrades an answer to ungrounded, which is reported, rather than
// denying the caller an answer at all.
func (o *Orchestrator) Answer(ctx context.Context, request Request) (Answer, error) {
	started := time.Now()
	mode, err := NormaliseMode(request.Assistant.Retrieval)
	if err != nil {
		return Answer{}, err
	}
	budget := o.Policy.MaxSteps
	if request.Assistant.MaxSteps > 0 && request.Assistant.MaxSteps < budget {
		budget = request.Assistant.MaxSteps
	}

	answer := Answer{Assistant: request.Assistant.Slug, Retrieval: mode}
	var steps []Step
	addStep := func(kind, summary, outcome string, detail map[string]any, latency time.Duration) {
		steps = append(steps, Step{
			Ordinal: len(steps), Kind: kind, Summary: summary, Outcome: outcome,
			Detail: detail, Latency: latency, Millis: latency.Milliseconds(),
		})
	}

	retrieve := ShouldRetrieve(mode, request.Question)
	addStep(StepPlan, fmt.Sprintf("mode %s, budget %d investigation steps", mode, budget), OutcomeSuccess,
		map[string]any{"retrieve": retrieve, "budget": budget}, 0)

	var hits []knowledge.Hit
	if retrieve {
		hits = o.gather(ctx, request, budget, addStep)
	} else {
		addStep(StepRetrieve, "skipped: the question is not a corpus query", OutcomeSkipped, nil, 0)
	}

	citations := Citations(hits)
	answer.Conflicted = Conflicted(citations, o.Policy.ConflictMargin)

	// Weak evidence is worse than none: passing low-scoring passages invites the
	// model to answer from them anyway.
	if BestScore(citations) < o.Policy.MinScore {
		if len(citations) > 0 {
			addStep(StepRetrieve, fmt.Sprintf("discarded %d weak passages, best score %.4f below %.4f",
				len(citations), BestScore(citations), o.Policy.MinScore), OutcomeSkipped, nil, 0)
		}
		hits, citations, answer.Conflicted = nil, nil, false
	}

	toolOutput, used := o.runTools(ctx, request, &steps, addStep)
	answer.ToolsUsed = used

	response, route, err := o.synthesise(ctx, request, hits, citations, toolOutput, answer.Conflicted, addStep)
	if err != nil {
		addStep(StepSynthesise, "model call failed", OutcomeFailure, nil, 0)
		answer.Steps = steps
		return answer, err
	}

	answer.Content = response.Content
	answer.Model = response.Model
	answer.Provider = route.Provider
	answer.Usage = response.Usage
	answer.Citations, answer.UsedCited = MarkUsed(response.Content, citations)
	answer.Grounded = len(answer.UsedCited) > 0
	answer.Hits = hits
	answer.LatencyMs = time.Since(started).Milliseconds()
	answer.Steps = steps
	return answer, nil
}

// investigationSteps counts the work a run has done.
//
// The plan record and the synthesis are not investigation: the plan is
// bookkeeping, and synthesis is what the caller asked for rather than something
// the agent chose to do. Counting either against the budget would silently give a
// three-step run one step of actual looking.
func investigationSteps(steps []Step) int {
	count := 0
	for _, step := range steps {
		switch step.Kind {
		case StepRetrieve, StepTool:
			// A skipped step consumed no budget.
			if step.Outcome != OutcomeSkipped {
				count++
			}
		}
	}
	return count
}

// gather runs retrieval rounds until the evidence is good enough or the budget
// runs out.
func (o *Orchestrator) gather(ctx context.Context, request Request, budget int,
	addStep func(kind, summary, outcome string, detail map[string]any, latency time.Duration)) []knowledge.Hit {

	readable, err := o.Classes.Readable(request.Caller.MaxClassification)
	if err != nil {
		addStep(StepRetrieve, "clearance could not be resolved", OutcomeFailure, nil, 0)
		return nil
	}
	access := knowledge.Access{
		CompanyID:       request.Caller.CompanyID,
		Department:      request.Caller.Department,
		Classifications: readable,
	}

	query := request.Question
	var all []knowledge.Hit

	for round := 0; round < budget; round++ {
		roundStart := time.Now()
		embeddings, err := o.Embedder.Embed(ctx, []string{query})
		if err != nil {
			addStep(StepRetrieve, "embedding provider unavailable", OutcomeFailure, nil, time.Since(roundStart))
			return all
		}
		found, err := o.Retriever.Search(ctx, query, embeddings[0], access, o.Policy.TopK)
		if err != nil {
			addStep(StepRetrieve, "corpus search failed", OutcomeFailure, nil, time.Since(roundStart))
			return all
		}

		all = append(all, found...)
		citations := Citations(all)
		best := BestScore(citations)
		addStep(StepRetrieve, fmt.Sprintf("round %d returned %d passages, best score %.4f",
			round+1, len(found), best), OutcomeSuccess,
			map[string]any{"passages": len(found), "bestScore": best, "distinct": len(citations)}, time.Since(roundStart))

		if best >= o.Policy.MinScore {
			return all
		}
		expanded, ok := ExpandQuery(request.Question, BestDocumentTitle(citations))
		if !ok {
			// Nothing to anchor a follow-up on; another identical search would
			// return the same passages.
			return all
		}
		query = expanded
	}
	return all
}

// runTools invokes the tools the caller asked for. A tool failure is reported in
// the trace and the answer continues without it.
func (o *Orchestrator) runTools(ctx context.Context, request Request, steps *[]Step,
	addStep func(kind, summary, outcome string, detail map[string]any, latency time.Duration)) (string, []string) {

	if len(request.Tools) == 0 || o.Tools == nil {
		return "", nil
	}

	var sections []string
	var used []string
	for _, slug := range request.Tools {
		if investigationSteps(*steps) >= o.Policy.MaxSteps {
			addStep(StepTool, "step budget exhausted before "+slug, OutcomeSkipped, nil, 0)
			break
		}
		callStart := time.Now()
		result, err := o.Tools.Invoke(ctx, "", slug, tool.Invocation{
			Caller: request.Caller, Arguments: map[string]any{"query": request.Question},
		})
		if err != nil {
			addStep(StepTool, slug+": "+err.Error(), OutcomeFailure, map[string]any{"tool": slug}, time.Since(callStart))
			continue
		}
		used = append(used, slug)
		sections = append(sections, slug+": "+result.Content)
		addStep(StepTool, slug+" returned "+fmt.Sprint(len([]rune(result.Content)))+" characters",
			OutcomeSuccess, map[string]any{"tool": slug}, time.Since(callStart))
	}
	return strings.Join(sections, "\n\n"), used
}

// synthesise builds the prompt and calls the model.
func (o *Orchestrator) synthesise(ctx context.Context, request Request, hits []knowledge.Hit,
	citations []Citation, toolOutput string, conflicted bool,
	addStep func(kind, summary, outcome string, detail map[string]any, latency time.Duration)) (provider.ChatResponse, provider.Route, error) {

	instructions := []string{}
	if request.Assistant.Instructions != "" {
		instructions = append(instructions, request.Assistant.Instructions)
	}
	if len(citations) > 0 {
		instructions = append(instructions,
			"Answer only from the numbered passages below. Cite every claim with its number in square brackets, "+
				"for example [1]. If the passages do not contain the answer, say so plainly instead of guessing.")
		if conflicted {
			instructions = append(instructions,
				"The passages come from different documents that appear to disagree. Present both positions and cite each.")
		}
	} else if request.Assistant.Retrieval == RetrievalAlways {
		// The assistant is configured to ground every answer and there is nothing
		// to ground it in. Saying so is the only correct behaviour.
		instructions = append(instructions,
			"No supporting passages were found. Say that the knowledge base does not cover this question "+
				"and do not answer from general knowledge.")
	}

	var body strings.Builder
	if len(citations) > 0 {
		body.WriteString("Passages:\n")
		body.WriteString(RenderContext(hits, citations))
		body.WriteString("\n\n")
	}
	if toolOutput != "" {
		body.WriteString("Tool results:\n")
		body.WriteString(toolOutput)
		body.WriteString("\n\n")
	}
	body.WriteString("Question: ")
	body.WriteString(request.Question)

	messages := make([]provider.Message, 0, 2)
	if len(instructions) > 0 {
		messages = append(messages, provider.Message{Role: "system", Content: strings.Join(instructions, "\n\n")})
	}
	messages = append(messages, provider.Message{Role: "user", Content: body.String()})

	callStart := time.Now()
	response, route, err := o.Completer.Chat(ctx, request.Assistant.LogicalModel, provider.ChatRequest{
		Messages:    messages,
		Temperature: request.Assistant.Temperature,
		MaxTokens:   request.Assistant.MaxTokens,
	})
	if err != nil {
		return provider.ChatResponse{}, route, err
	}

	addStep(StepSynthesise, fmt.Sprintf("answered with %d passages offered, %d tokens",
		len(citations), response.Usage.TotalTokens), OutcomeSuccess,
		map[string]any{"passages": len(citations), "tokens": response.Usage.TotalTokens}, time.Since(callStart))
	return response, route, nil
}
