.PHONY: test clean

test: traceProcFillerStap.go traceProcFillerStap_test.go traceProcMonitor.go
	go test -c -timeout 20m -v -race $^
	./goexecsnoop.test -test.failfast -test.v

clean:
	rm -fv main.test
