// The portal's views and the shape of a request to move between them.
//
// This lives apart from the shell so a page can navigate without importing the
// shell that renders it.

export type View =
  | 'home'
  | 'chat'
  | 'history'
  | 'assistants'
  | 'documents'
  | 'search'
  | 'favorites'
  | 'shared'
  | 'settings'
  | 'portal'
  | 'roadmap'
  | 'rules'
  | 'architecture';

/** Which conversation, or which assistant, the chat page should open with. */
export type ChatTarget = { conversationId?: string; assistant?: string };

export type Navigate = (view: View, target?: ChatTarget) => void;
