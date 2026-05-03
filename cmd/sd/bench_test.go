package main

import (
	"bytes"
	"testing"
	"time"
)

func BenchmarkPrintInputHistory(b *testing.B) {
	entries := make([]inputHistoryEntry, 0, 2000)
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 2000; i++ {
		entries = append(entries, inputHistoryEntry{
			ID:        i + 1,
			Timestamp: start.Add(time.Duration(i) * time.Second),
			SessionID: "s1",
			Hidden:    i%13 == 0,
			Text:      "benchmark input line for history rendering",
		})
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		printInputHistory(&out, entries)
	}
}

func BenchmarkWrapWordsNoSplit(b *testing.B) {
	text := "sd benchmark wraps long lines without splitting words and keeps readability under constrained widths"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = wrapWordsNoSplit(text, 40)
	}
}
