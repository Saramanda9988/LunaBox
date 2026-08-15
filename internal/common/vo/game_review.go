package vo

// GameReviewProviderSyncResult describes one provider's review sync outcome.
type GameReviewProviderSyncResult struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Error    string `json:"error"`
}

// GameReviewSyncResult keeps provider outcomes separate so partial success is visible.
type GameReviewSyncResult struct {
	Results   []GameReviewProviderSyncResult `json:"results"`
	Succeeded int                            `json:"succeeded"`
	Failed    int                            `json:"failed"`
}
