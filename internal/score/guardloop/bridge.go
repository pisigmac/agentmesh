package guardloop

// ScoreBridge is kept for backward-compatible wiring; Client already implements score.Provider.
type ScoreBridge struct{ *Client }
