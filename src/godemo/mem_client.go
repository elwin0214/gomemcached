package main

import (
	"flag"
	"fmt"
	. "logger"
	. "memcached"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
	"util"
)

func main() {
	address := flag.String("a", "127.0.0.1:8080", "server listen port")
	conns := flag.Int("cn", 1, "the number of tcp connection")
	vl := flag.Int("vl", 10, "the length of value")
	clients := flag.Int("cs", 10, "the number of clients")
	requests := flag.Int("reqs", 100, "the number of requests")
	level := flag.Int("l", 2, "log level")
	flag.Parse()

	LOG.SetHandler(NewStreamHandler(os.Stdout))
	LOG.SetLevel(*level)
	LOG.Warn("maxprocs = %d\n", runtime.GOMAXPROCS(0))
	LOG.Warn("cpuprof = %s\n", *cpuprof)

	cf, _ := os.Create("cpu.out")
	pprof.StartCPUProfile(cf)
	defer pprof.StopCPUProfile()

	buf := make([]byte, 0, *vl)
	for j := 0; j < *vl; j++ {
		buf = append(buf, 'a')
	}
	value := string(buf)
	LOG.Info("value = %s\n", value)
	c := NewMemcachedClient(*address, *conns, 5000)
	c.Start()
	var wg sync.WaitGroup
	wg.Add(*clients)
	stat := util.NewStat(1024*1024, 0, 100)
	stat.Start()
	start := time.Now()
	for i := 0; i < *clients; i++ {
		go func(index int) {
			LOG.Info("start %d\n", index)

			defer wg.Done()
			for k := 0; k < *requests; k++ {
				key := fmt.Sprintf("%d_%d", index, k)
				s := time.Now()
				r, err := c.Set(key, value)
				e := time.Now()
				elasp := int(e.Sub(s) / 1000 / 1000)
				stat.Collect(elasp)
				LOG.Info("key = %s value = %s result = %t err = %v elaspe = %d\n", key, value, r, err, elasp)

			}
			LOG.Info("asd")
		}(i)
	}
	wg.Wait()
	end := time.Now()
	LOG.Warn("[main] client = %d requests = %d time = %dms\n", *clients, *requests, end.Sub(start)/1000/1000)
	time.Sleep(1000 * time.Millisecond)

	stat.Close()
	LOG.Warn(stat.View())

	time.Sleep(10000 * time.Millisecond)
	c.Close()
}
