# DA GLOSSARY

## STREAM

a sequence of values over time

* a resource's versions
* a job's input sets
* a job's output sets

## CHAIN

a stream that continuously generates new values based on the last result

with a sleep(60)

* continuous resource checking

## AGGREGATE

take a set of named streams and return a single stream of named values

* a job's un-constrained resource inputs

## INTERSECT

take multiple streams of named values and return only values that are present
across all streams

* a job's passed: constrained inputs

## PUBSUB

a stream that allows multiple subscribers to receive the same values

* how jobs communicate their inputs to each other
* a job subscribes to its inputs and publishes only input sets that
  successfully run a build
