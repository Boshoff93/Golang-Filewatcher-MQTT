package main

import (
	"fmt"
	"log"
	"time"
  "bufio"
	"github.com/radovskyb/watcher"
  "os"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var f MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
  fmt.Printf("TOPIC: %s\n", msg.Topic())
  fmt.Printf("MSG: %s\n", msg.Payload())
}

func main() {
	opts := MQTT.NewClientOptions().AddBroker("tcp://127.0.0.1:1883")
  opts.SetClientID("go-simple")
  opts.SetDefaultPublishHandler(f)

  //Create and start a client using the above ClientOptions
  c := MQTT.NewClient(opts)
  if token := c.Connect(); token.Wait() && token.Error() != nil {
    panic(token.Error())
  }
	if token := c.Subscribe("MCITOPIC", 0, nil); token.Wait() && token.Error() != nil {
    fmt.Println(token.Error())
    os.Exit(1)
  }
	w := watcher.New()

	// SetMaxEvents to 1 to allow at most 1 event's to be received on the Event channel per watching cycle.
	// If SetMaxEvents is not set, the default is to send all events.
	w.SetMaxEvents(1)

	// Only notify rename and move events.
	w.FilterOps(watcher.Write)

	go func() {
		for {
			select {
			case <-w.Event:
        file, err := os.Open("test_folder/file.txt")
        if err != nil {
          panic(err)
        }
        defer file.Close()

        var lines []string
        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
          lines = append(lines, scanner.Text())
        }
        if err := scanner.Err(); err != nil {
          fmt.Fprintln(os.Stderr, err)
        }
				text := lines[len(lines)-1]
		    token := c.Publish("MCITOPIC", 0, false, text)
				token.Wait()

			case err := <-w.Error:
				log.Fatalln(err)
			case <-w.Closed:
				return
			}
		}
	}()

	// Watch this folder for changes.
	if err := w.Add("."); err != nil {
		log.Fatalln(err)
	}

	// Watch test_folder recursively for changes.
	if err := w.AddRecursive("./test_folder"); err != nil {
		log.Fatalln(err)
	}

	// Print a list of all of the files and folders currently
	// being watched and their paths.
	for path, f := range w.WatchedFiles() {
		fmt.Printf("%s: %s\n", path, f.Name())
	}

	fmt.Println()

	// Trigger 2 events after watcher started.
	go func() {
		w.Wait()
	}()

	// Start the watching process - it'll check for changes every 100ms.
	if err := w.Start(time.Millisecond * 100); err != nil {
		log.Fatalln(err)
	}
}
