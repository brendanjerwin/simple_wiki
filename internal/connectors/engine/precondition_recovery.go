package engine

// Precondition recovery is the engine's response to an
// ErrorClassPreconditionFailed return from Adapter.PatchRemote. Per
// MATRIX.md row 6 (and the Tasks v2 implementation that becomes
// engine policy under strictest-behavior-wins), the recovery has
// three branches:
//
//   - 412_remote_deleted: Adapter.ReadRemoteByRef returns
//     RemoteItem{Deleted: true} or NotFound. The remote item was
//     deleted between our last observation and the patch attempt.
//     Mirror the deletion into the wiki via mutator.DeleteItemForSync
//     (with suppressor); clean up id_map and etag entries in
//     AdapterState.
//
//   - 412_remote_unchanged_repatch: Adapter.ReadRemoteByRef returns
//     a RemoteItem whose fields match the last successfully-pushed
//     SyncedItems baseline (or the equivalent in AdapterState). The
//     412 was phantom — etag desync, sequencing race, server-side
//     internal bump. Refresh the etag from the read response and
//     re-PATCH with fresh ItemEtags. The user's wiki edit wins.
//
//   - 412_remote_authoritative_apply: Adapter.ReadRemoteByRef returns
//     a RemoteItem whose fields differ from our baseline. The remote
//     really did move under us. Translate the remote via
//     Adapter.RemoteToWiki and apply via the mutator (with
//     suppressor). The next tick's reconcile re-evaluates the diff
//     against the now-authoritative wiki state.
//
// Phase 2 status: stub. Phase 3 wires this in as a recovery branch
// invoked from reconcile.go's outbound push path.
//
// Keep's stage3 HTTP 500 on stale/missing baseVersion maps to
// ErrorClassPreconditionFailed via Keep's adapter.ClassifyError;
// the same 3 branches apply to Keep with no per-adapter override.
// This is the operationalization of "strictest-behavior-wins."
