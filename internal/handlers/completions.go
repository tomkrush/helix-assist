package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/leona/helix-assist/internal/config"
	"github.com/leona/helix-assist/internal/lsp"
	"github.com/leona/helix-assist/internal/providers"
	"github.com/leona/helix-assist/internal/util"
)

type CompletionHandler struct {
	cfg        *config.Config
	registry   *providers.Registry
	debouncer  *util.Debouncer
	mu         sync.Mutex
	generation uint64
	active     *activeCompletion
}

type activeCompletion struct {
	generation uint64
	cancel     context.CancelFunc
}

func NewCompletionHandler(cfg *config.Config, registry *providers.Registry) *CompletionHandler {
	return &CompletionHandler{
		cfg:       cfg,
		registry:  registry,
		debouncer: util.NewDebouncer(),
	}
}

func (h *CompletionHandler) Register(svc *lsp.Service) {
	svc.On(lsp.EventCompletion, func(svc *lsp.Service, msg *lsp.JSONRPCMessage) {
		var params lsp.CompletionParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			svc.Logger.Log("completion parse error:", err.Error())
			return
		}

		buffer, ok := svc.Buffers.Get(params.TextDocument.URI)
		if !ok {
			h.sendEmptyCompletion(svc, msg.ID)
			return
		}

		generation := h.beginCompletion()
		lastContentVersion := buffer.Version

		h.debouncer.Debounce(
			"completion",
			func() {
				h.doCompletion(svc, msg, params, lastContentVersion, generation)
			},
			func() {
				h.sendEmptyCompletion(svc, msg.ID)
			},
			time.Duration(h.cfg.Debounce)*time.Millisecond,
		)
	})
}

func (h *CompletionHandler) beginCompletion() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.generation++
	if h.active != nil {
		h.active.cancel()
		h.active = nil
	}
	return h.generation
}

func (h *CompletionHandler) setActive(generation uint64, cancel context.CancelFunc) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if generation != h.generation {
		return false
	}
	h.active = &activeCompletion{generation: generation, cancel: cancel}
	return true
}

func (h *CompletionHandler) clearActive(generation uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.active != nil && h.active.generation == generation {
		h.active = nil
	}
}

func (h *CompletionHandler) isCurrent(generation uint64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return generation == h.generation
}

func (h *CompletionHandler) doCompletion(svc *lsp.Service, msg *lsp.JSONRPCMessage, params lsp.CompletionParams, lastContentVersion int, generation uint64) {
	defer func() {
		if r := recover(); r != nil {
			svc.Logger.Log("completion panic:", r)
			h.sendEmptyCompletion(svc, msg.ID)
		}
	}()

	buffer, ok := svc.Buffers.Get(params.TextDocument.URI)
	if !ok {
		h.sendEmptyCompletion(svc, msg.ID)
		return
	}

	if buffer.Version > lastContentVersion {
		svc.Logger.Log("skipping completion - content is stale")
		h.sendEmptyCompletion(svc, msg.ID)
		return
	}

	content := util.GetContent(buffer.Text, params.Position.Line, params.Position.Character)
	svc.Logger.Log("calling completion", "language:", buffer.LanguageID)

	var progress *util.ProgressIndicator

	if h.cfg.EnableProgressSpinner {
		progress = util.NewProgressIndicator(svc, h.cfg)
		progress.Start()
		defer progress.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.cfg.CompletionTimeout)*time.Millisecond)
	if !h.setActive(generation, cancel) {
		cancel()
		h.sendEmptyCompletion(svc, msg.ID)
		return
	}
	defer cancel()
	defer h.clearActive(generation)
	contentAfter := content.ContentImmediatelyAfter

	if content.ContentAfter != "" {
		if contentAfter != "" {
			contentAfter += "\n" + content.ContentAfter
		} else {
			contentAfter = content.ContentAfter
		}
	}

	hints, err := h.registry.Completion(ctx, providers.CompletionRequest{
		ContentBefore: content.ContentBefore,
		ContentAfter:  contentAfter,
	}, params.TextDocument.URI, buffer.LanguageID, h.cfg.NumSuggestions)

	if err != nil {
		if errors.Is(err, context.Canceled) || !h.isCurrent(generation) {
			h.sendEmptyCompletion(svc, msg.ID)
			return
		}
		svc.Logger.Log("completion error:", err.Error())
		svc.SendDiagnostics([]lsp.Diagnostic{
			{
				Message:  err.Error(),
				Severity: lsp.SeverityError,
				Range: lsp.Range{
					Start: lsp.Position{Line: params.Position.Line, Character: 0},
					End:   lsp.Position{Line: params.Position.Line + 1, Character: 0},
				},
			},
		}, 0)
		h.sendEmptyCompletion(svc, msg.ID)
		return
	}

	buffer, ok = svc.Buffers.Get(params.TextDocument.URI)
	if !ok || buffer.Version != lastContentVersion || !h.isCurrent(generation) {
		svc.Logger.Log("discarding stale completion result")
		h.sendEmptyCompletion(svc, msg.ID)
		return
	}

	svc.Logger.Log("completion hints:", len(hints))

	items := make([]lsp.CompletionItem, 0, len(hints))
	for _, hint := range hints {
		item := h.buildCompletionItem(hint, content, params.Position)
		items = append(items, item)
	}

	svc.Send(&lsp.JSONRPCMessage{
		ID: msg.ID,
		Result: lsp.CompletionList{
			IsIncomplete: false,
			Items:        items,
		},
	})
}

func findOverlapSuffix(hint, suffix string) int {
	if suffix == "" {
		return 0
	}

	hint = strings.TrimRight(hint, " \t")
	maxOverlap := len(hint)

	if len(suffix) < maxOverlap {
		maxOverlap = len(suffix)
	}

	for i := maxOverlap; i > 0; i-- {
		hintSuffix := hint[len(hint)-i:]
		suffixPrefix := suffix[:i]

		if hintSuffix == suffixPrefix {
			return i
		}
	}

	return 0
}

func utf16Length(text string) int {
	return len(utf16.Encode([]rune(text)))
}

func completionWordPrefix(line string) string {
	runes := []rune(line)
	start := len(runes)
	for start > 0 {
		r := runes[start-1]
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' && r != '$' && r != '-' {
			break
		}
		start--
	}
	return string(runes[start:])
}

func inProseContext(contentBefore string) bool {
	line := contentBefore
	if newline := strings.LastIndexByte(line, '\n'); newline >= 0 {
		line = line[newline+1:]
	}

	for _, marker := range []string{"//", "#", "<!--"} {
		if strings.Contains(line, marker) {
			return true
		}
	}

	if strings.LastIndex(contentBefore, "/*") > strings.LastIndex(contentBefore, "*/") ||
		strings.LastIndex(contentBefore, "<!--") > strings.LastIndex(contentBefore, "-->") {
		return true
	}

	for _, quote := range []rune{'"', '\'', '`'} {
		open := false
		escaped := false
		for _, r := range line {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' && quote != '`' {
				escaped = true
				continue
			}
			if r == quote {
				open = !open
			}
		}
		if open {
			return true
		}
	}

	return false
}

func addMissingProseSpace(hint string, content util.ContentParts) string {
	if hint == "" || content.ContentBefore == "" || !inProseContext(content.ContentBefore) {
		return hint
	}

	beforeRunes := []rune(content.ContentBefore)
	hintRunes := []rune(hint)
	left := beforeRunes[len(beforeRunes)-1]
	right := hintRunes[0]
	if (unicode.IsLetter(left) || unicode.IsNumber(left)) && (unicode.IsLetter(right) || unicode.IsNumber(right)) {
		return " " + hint
	}
	return hint
}

func (h *CompletionHandler) buildCompletionItem(hint string, content util.ContentParts, position lsp.Position) lsp.CompletionItem {
	// Remove response framing newlines without removing meaningful horizontal
	// whitespace. A leading space is often the entire point of a prose/string
	// continuation (for example, `"hello<CURSOR>"` -> ` world`).
	hint = strings.Trim(hint, "\r\n")

	lastLineTrimmed := strings.TrimSpace(content.LastLine)

	if strings.HasPrefix(hint, lastLineTrimmed) {
		hint = hint[len(lastLineTrimmed):]
	}
	hint = addMissingProseSpace(hint, content)

	lines := strings.Split(hint, "\n")

	label := lines[0]
	prefix := completionWordPrefix(content.LastLine)
	if prefix != "" && !strings.HasPrefix(label, prefix) {
		label = prefix + label
	}

	overlapBytes := findOverlapSuffix(hint, content.ContentImmediatelyAfter)

	replaceLen := utf16Length(content.ContentImmediatelyAfter[:overlapBytes])

	if overlapBytes == 0 && content.ContentImmediatelyAfter != "" {
		firstChar := content.ContentImmediatelyAfter[0]

		if firstChar == ')' || firstChar == '}' || firstChar == ']' || firstChar == '>' {
			restOfLine := content.ContentImmediatelyAfter[1:]
			isIsolated := len(content.ContentImmediatelyAfter) == 1 ||
				len(strings.TrimLeft(restOfLine, " \t")) == 0

			if isIsolated {
				replaceLen = 1
			}
		}
	}

	// Helix's default handling for insertText replaces the word under the
	// cursor. The provider returns only text to add, so use an explicit edit
	// beginning at the cursor. It may consume an already-present delimiter,
	// but it never consumes the typed prefix before the cursor.
	textEdit := &lsp.TextEdit{
		Range: lsp.Range{
			Start: position,
			End: lsp.Position{
				Line:      position.Line,
				Character: position.Character + replaceLen,
			},
		},
		NewText: hint,
	}

	return lsp.CompletionItem{
		Label:     label,
		Kind:      1,
		Preselect: true,
		Detail:    hint,
		TextEdit:  textEdit,
		SortText:  "00000",
	}
}

func (h *CompletionHandler) sendEmptyCompletion(svc *lsp.Service, id *int) {
	svc.Send(&lsp.JSONRPCMessage{
		ID: id,
		Result: lsp.CompletionList{
			IsIncomplete: false,
			Items:        []lsp.CompletionItem{},
		},
	})
}
