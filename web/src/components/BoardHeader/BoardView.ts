/** The three top-level views of the panel. The ids double as the tab labels. */
export const BOARD_VIEWS = ['board', 'standup', 'tokens'] as const

export type BoardView = (typeof BOARD_VIEWS)[number]
