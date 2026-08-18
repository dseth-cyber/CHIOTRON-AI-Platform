import type { ReactNode } from 'react';

/**
 * What a page shows when it has nothing to show.
 *
 * An empty result and a missing permission look identical if a page just renders
 * nothing, so every empty state says which one it is and, where there is one,
 * offers the action that would fill it.
 */
export function EmptyState({
  title,
  body,
  action,
}: {
  title: string;
  body: string;
  action?: ReactNode;
}) {
  return (
    <section className="empty-state">
      <h2>{title}</h2>
      <p>{body}</p>
      {action}
    </section>
  );
}

/** A one-line badge used for classifications, statuses and scopes. */
export function Tag({ tone, children }: { tone?: string; children: ReactNode }) {
  return <span className={tone ? `tag tone-${tone}` : 'tag'}>{children}</span>;
}
