bench/cache/old:
	go test  -benchmem -count=2 .

bench/cache/old:
	go test -bench=BenchmarkBroadcast -benchmem -count=10 . | tee old.txt
bench/cache/new:
	go test -bench=BenchmarkBroadcast -benchmem -count=10 . | tee new.txt
bench/cache/stat:
	go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt

fieldalignment:
	go run golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest -fix ./...
