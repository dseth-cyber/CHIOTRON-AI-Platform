// The portal's views and the shape of a request to move between them.
//
// This lives apart from the shell so a page can navigate without importing the
// shell that renders it.

export type DetailKind =
  | 'rules'
  | 'architecture'
  | 'capabilities'
  | 'blueprint'
  | 'api'
  | 'module'
  | 'event'
  | 'map'
  | 'flags'
  | 'prompts';

export type View =
  | 'home'
  | 'chat'
  | 'analyze'
  | 'create'
  | 'history'
  | 'assistants'
  | 'documents'
  | 'search'
  | 'favorites'
  | 'shared'
  | 'providers'
  | 'settings'
  | 'portal'
  | 'roadmap'
  | DetailKind;

/** Which conversation, or which assistant, the chat page should open with. */
export type ChatTarget = { conversationId?: string; assistant?: string; prompt?: string };

export type Navigate = (view: View, target?: ChatTarget) => void;
