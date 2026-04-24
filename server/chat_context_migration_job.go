package server

import (
	"fmt"
	"log/slog"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// ChatContextMigrationJobName is the queue name used by the one-time
// migration job that moves the legacy ai_agent_chat_context frontmatter key
// under the new agent.chat_context namespace.
const ChatContextMigrationJobName = "ChatContextMigration"

const legacyChatContextKey = "ai_agent_chat_context"

// ChatContextMigrationJob walks every page that still has the legacy
// ai_agent_chat_context key in frontmatter and rewrites it under
// agent.chat_context. The job is idempotent — a second run finds no
// remaining legacy keys (because the index reflects the post-migration
// state) and is a no-op. Pages where agent.chat_context already exists are
// preserved unchanged on the destination side; the legacy key is still
// removed so the next run is fast.
type ChatContextMigrationJob struct {
	pages agentSchedulePagesStore
	index AgentScheduleQueryer
}

// NewChatContextMigrationJob constructs a migration job.
func NewChatContextMigrationJob(pages agentSchedulePagesStore, index AgentScheduleQueryer) *ChatContextMigrationJob {
	return &ChatContextMigrationJob{pages: pages, index: index}
}

// GetName implements jobs.Job.
func (*ChatContextMigrationJob) GetName() string {
	return ChatContextMigrationJobName
}

// Execute implements jobs.Job. Walks every page whose frontmatter still
// includes the legacy chat-context key and rewrites it.
func (j *ChatContextMigrationJob) Execute() error {
	pages := j.index.QueryKeyExistence(legacyChatContextKey)
	if len(pages) == 0 {
		return nil
	}
	slog.Info("ChatContextMigration: starting", "page_count", len(pages))

	migrated := 0
	for _, page := range pages {
		moved, err := j.migrateOne(string(page))
		if err != nil {
			slog.Error("ChatContextMigration: failed", "page", page, "error", err)
			continue
		}
		if moved {
			migrated++
		}
	}
	slog.Info("ChatContextMigration: complete", "migrated", migrated, "scanned", len(pages))
	return nil
}

// migrateOne rewrites a single page. Returns true when a write occurred.
func (j *ChatContextMigrationJob) migrateOne(page string) (bool, error) {
	id, fm, err := j.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		return false, fmt.Errorf("read %q: %w", page, err)
	}

	legacy, hasLegacy := fm[legacyChatContextKey]
	if !hasLegacy {
		return false, nil
	}

	moveLegacyChatContextLocked(fm, legacy)

	if err := j.pages.WriteFrontMatter(id, fm); err != nil {
		return false, fmt.Errorf("write %q: %w", page, err)
	}
	return true, nil
}

// moveLegacyChatContextLocked promotes the legacy ai_agent_chat_context value
// to agent.chat_context and removes the legacy key. If agent.chat_context
// already exists on the page (e.g. the user manually started using the new
// schema), we keep the existing destination — do not clobber. The legacy
// key is removed regardless so subsequent runs see clean data.
func moveLegacyChatContextLocked(fm wikipage.FrontMatter, legacy any) {
	delete(fm, legacyChatContextKey)

	agent, ok := fm[AgentNamespaceKey].(map[string]any)
	if !ok {
		agent = map[string]any{}
	}
	if _, alreadyMigrated := agent[agentChatContextKey]; !alreadyMigrated {
		agent[agentChatContextKey] = legacy
	}
	fm[AgentNamespaceKey] = agent
}
