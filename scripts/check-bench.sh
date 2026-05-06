#!/usr/bin/env bash
set -euo pipefail

MAX_PRINT_INPUT_HISTORY_NS="${MAX_PRINT_INPUT_HISTORY_NS:-250000000}"
MAX_WRAP_WORDS_NS="${MAX_WRAP_WORDS_NS:-10000000}"

output="$(go test ./cmd/respec -run '^$' -bench 'Benchmark(PrintInputHistory|WrapWordsNoSplit)$' -benchmem -count=1)"
printf '%s\n' "$output"

extract_ns() {
  local name="$1"
  awk -v bench="$name" '
    $1 ~ bench {
      for (i = 1; i <= NF; i++) {
        if ($i == "ns/op") {
          print $(i-1)
          exit
        }
      }
    }
  ' <<<"$output"
}

print_input_ns="$(extract_ns 'BenchmarkPrintInputHistory')"
wrap_words_ns="$(extract_ns 'BenchmarkWrapWordsNoSplit')"

if [[ -z "$print_input_ns" || -z "$wrap_words_ns" ]]; then
  echo "benchmark thresholds check failed: could not parse benchmark output"
  exit 1
fi

awk -v value="$print_input_ns" -v max="$MAX_PRINT_INPUT_HISTORY_NS" 'BEGIN { exit(value <= max ? 0 : 1) }' || {
  echo "BenchmarkPrintInputHistory too slow: ${print_input_ns} ns/op (max ${MAX_PRINT_INPUT_HISTORY_NS})"
  exit 1
}

awk -v value="$wrap_words_ns" -v max="$MAX_WRAP_WORDS_NS" 'BEGIN { exit(value <= max ? 0 : 1) }' || {
  echo "BenchmarkWrapWordsNoSplit too slow: ${wrap_words_ns} ns/op (max ${MAX_WRAP_WORDS_NS})"
  exit 1
}

echo "benchmark threshold checks passed"

