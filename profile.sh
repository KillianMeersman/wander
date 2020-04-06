#!/bin/bash

FILENAME=$(basename $(pwd))
go test -run=. -bench=. -cpuprofile=cpu.out -benchmem -memprofile=mem.out -trace trace.out
go tool pprof -pdf $FILENAME.test cpu.out > cpu.pdf && open cpu.pdf
go tool pprof -pdf --alloc_space $FILENAME.test mem.out > alloc_space.pdf && open alloc_space.pdf
go tool pprof -pdf --alloc_objects $FILENAME.test mem.out > alloc_objects.pdf && open alloc_objects.pdf
go tool pprof -pdf --inuse_space $FILENAME.test mem.out > inuse_space.pdf && open inuse_space.pdf
go tool pprof -pdf --inuse_objects $FILENAME.test mem.out > inuse_objects.pdf && open inuse_objects.pdf
go tool trace trace.out

rm *.pdf *.out wander.test