# Process data aggregator challenge

### Motivation.

Build a golang tool to aggregate data coming from an arbitrary process, and calculate the sum and average of cpu and memory in a thread safe way.

### execution

```
go build -o aggregator ./challenge/src 

./aggregtor

```

Optional command line params,

```
-i interval to display the aggregation, defaults to 1 minute

-n number of apps to run, defaults to 10

-s queue size for the buffered channel, defaults to 10
```


_TODO_

* Tests