package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	// make a buffered channel with size 1 for handling the os signals
	signalChan := make(chan os.Signal, 1)
	// defer the close
	defer close(signalChan)
	signal.Notify(signalChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGABRT,
		syscall.SIGKILL,
		syscall.SIGSEGV,
		syscall.SIGSYS,
		syscall.SIGPIPE,
		syscall.SIGTERM,
	)
	var printInterval time.Duration
	var numApps, queueSize int
	// default time interval for display is 1 minute
	flag.DurationVar(&printInterval, "i", time.Minute*1, "Denotes how often should we print the results.")
	flag.IntVar(&numApps, "n", 10, "Denotes how many apps should we run")
	flag.IntVar(&queueSize, "s", 10, "Denotes the size of the channel queue")
	flag.Parse()

	// create a buffered channel for queue
	dataChan := make(chan DataPoint, queueSize)
	queue := Queue{
		dataChan,
	}
	state := NewState()
	for i := 1; i <= numApps; i++ {
		appName := "App" + strconv.Itoa(i)
		producer := NewProducer(appName, &queue)
		go producer.ProduceDataPoint()
		fmt.Printf("Scheduled %s for producing..\n", appName)
	}
	go func() {
		for {
			select {
			case <-time.After(printInterval):
				{
					state.print()
				}
			}
		}
	}()
	// await for the signals to this channel
	consumer := NewAggregator(&queue, &state)
	go consumer.Consume()
	<-signalChan

}

// Set sets the data point for the app to the state structure
func (state *State) Set(newDataPoint DataPoint) {
	// check if the value already exists in the map and add if it doesnt

	if existingValue, present := state.metrics[newDataPoint.appName]; present {
		existingValue.memSamples.Add(newDataPoint.memory)
		existingValue.cpuSamples.Add(newDataPoint.cpu)
	} else {
		mem := NewMemory()
		cpu := NewCPU()
		mem.Add(newDataPoint.memory)
		cpu.Add(newDataPoint.cpu)
		dataPoints := DataPoints{
			memSamples: mem,
			cpuSamples: cpu,
		}
		state.metrics[newDataPoint.appName] = dataPoints
	}
}

// assuming the app name always be AppX
func appIdx(appName string) int {
	split := strings.Split(appName, "App")
	id, _ := strconv.Atoi(split[1])
	return id
}

func (state *State) print() {

	// crude way to sort the map
	sorted := make([]string, len(state.metrics))
	i := 0
	for k := range state.metrics {
		sorted[i] = k
		i++
	}
	sort.Slice(sorted, func(i, j int) bool {
		return appIdx(sorted[i]) < appIdx(sorted[j])
	})
	// at this point we should have the sorted keys in the sorted slice
	fmt.Print("\n\n=================Aggregation=================\n\n")
	for _, item := range sorted {
		sumOfCPU := state.metrics[item].cpuSamples.Sum
		sumOfMemory := state.metrics[item].memSamples.Sum
		lengthOfCPUItems := float64(len(state.metrics[item].cpuSamples.Samples))
		avgOfCPU := sumOfCPU / lengthOfCPUItems
		fmt.Printf("<%s , %d, %.2f>\n", item, sumOfMemory, avgOfCPU)
	}
	fmt.Print("\n\n=================end=================\n\n")
}

// ProduceDataPoint produces the data point
func (p *Producer) ProduceDataPoint() {
	// define a max upperlimit in seconds
	upperLimit := int64(5)
	// generate random data points
	go func() {
		for {
			randomMem := rand.Intn(2000)
			randomCPU := rand.Float64()
			randomInterval := time.Duration(rand.Int63n(upperLimit)) * time.Second
			select {
			case <-time.After(randomInterval):
				// produce after the random interval
				// fmt.Printf("Producing data for %s as <%s, %d, %f>\n", p.appName, p.appName, randomMem, randomCPU)
				p.queue.channel <- DataPoint{
					appName: p.appName,
					memory:  randomMem,
					cpu:     randomCPU,
				}
			}
		}

	}()
}

// NewProducer new producer constructor
// pointer to queue is passed here since we're going to be sending the data to the same queue
func NewProducer(appName string, queue *Queue) Producer {
	return Producer{
		appName: appName,
		queue:   queue,
	}
}

// NewAggregator creates a new aggregator
func NewAggregator(queue *Queue, state *State) Aggregator {
	return Aggregator{
		queue: queue,
		state: state,
	}
}

// NewState creates a new state and inits
func NewState() State {
	return State{
		metrics: make(map[string]DataPoints, 0),
	}
}

// Consume consumes the data point and persists
func (c *Aggregator) Consume() {

	//Process message from the queue
	//Persist data in the state
	for {
		select {
		case newDataPoint := <-c.queue.channel:
			// only considering the case where app names to start with App
			fmt.Printf("Data point received for %s, with data <%s, %d, %f>\n", newDataPoint.appName, newDataPoint.appName, newDataPoint.memory, newDataPoint.cpu)
			c.state.rwLock.Lock()
			c.state.Set(newDataPoint)
			c.state.rwLock.Unlock()

		}

	}

}

// CPU for adding the cpu
type CPU struct {
	Samples []float64
	rwLock  *sync.RWMutex
	Sum     float64
}

// Memory describes memory
type Memory struct {
	Sum     int
	samples []int
	rwLock  *sync.RWMutex
}

// NewMemory creates a new memory
func NewMemory() *Memory {
	return &Memory{
		Sum:     0,
		samples: make([]int, 0),
		rwLock:  &sync.RWMutex{},
	}
}

// NewCPU creates a new cpu
func NewCPU() *CPU {
	return &CPU{
		Sum:     0,
		Samples: make([]float64, 0),
		rwLock:  &sync.RWMutex{},
	}
}

// Add adds a new item to the mem struct
func (mem *Memory) Add(newValue int) {
	mem.rwLock.Lock()
	mem.Sum += newValue
	mem.samples = append(mem.samples, newValue)
	mem.rwLock.Unlock()
}

// Add adds a new item to the cpu struct
func (cpu *CPU) Add(newValue float64) {
	cpu.rwLock.Lock()
	cpu.Sum += newValue
	cpu.Samples = append(cpu.Samples, newValue)
	cpu.rwLock.Unlock()

}

// DataPoint represents a datapoint from an app
type DataPoint struct {
	appName string
	memory  int
	cpu     float64
}

// DataPoints composition
type DataPoints struct {
	memSamples *Memory
	cpuSamples *CPU
}

// Queue represents the channel for sending and receiving DataPoints
type Queue struct {
	channel chan DataPoint
}

// Producer represents the producer which produces new data points to the queue.
type Producer struct {
	appName string
	queue   *Queue
}

// Aggregator represents the struct which aggregates the data points aka consumer
type Aggregator struct {
	queue *Queue
	state *State
}

// State representation
type State struct {
	metrics map[string]DataPoints
	rwLock  sync.RWMutex
}
