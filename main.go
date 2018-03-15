package main

import (
	"fmt"
	"log"
	"time"
  "bufio"
	"github.com/radovskyb/watcher"
  "os"
	"strings"
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

	tokenStart := c.Publish("MCITOPIC", 0, false, "(S)")
	tokenStart.Wait()

	w1 := watcher.New()
	w2 := watcher.New()
	w3 := watcher.New()

	// SetMaxEvents to 1 to allow at most 1 event's to be received on the Event channel per watching cycle.
	// If SetMaxEvents is not set, the default is to send all events.
	w1.SetMaxEvents(1)
	w2.SetMaxEvents(1)
	w3.SetMaxEvents(1)

	// Only notify rename and move events.
	w1.FilterOps(watcher.Write)
	w2.FilterOps(watcher.Write)
	w3.FilterOps(watcher.Write)

	var masterFileName string
	participantFolder, err := os.Open(".")
    if err != nil {
        log.Fatalf("failed opening directory: %s", err)
    }
    defer participantFolder.Close()

    list1,_ := participantFolder.Readdirnames(0) // 0 to read all files and folders
    for _, name := range list1 {
        endsWith1 := strings.HasSuffix(name, "_Master_Event_Log.res")
				if(endsWith1) {
					masterFileName = name;
				}
    }

		var resourceFileName string
		var trackingFileName string

		conditionFolder, err := os.Open("./Condition_1")
	    if err != nil {
	        log.Fatalf("failed opening directory: %s", err)
	    }
	    defer conditionFolder.Close()

	    list2,_ := conditionFolder.Readdirnames(0) // 0 to read all files and folders
	    for _, name := range list2 {
	        endsWith2 := strings.HasSuffix(name, "_Resource_Management_Log_1.res")
					if(endsWith2) {
						resourceFileName = name;
					}
					endsWith3 := strings.HasSuffix(name, "_Tracking_Log_1.res")
					if(endsWith3) {
						trackingFileName = name;
					}
	    }

	go func() {
		for {
			select {
			case <-w1.Event:
				fmt.Println(masterFileName + " this is the name")
        masterFile, err := os.Open(masterFileName)
        if err != nil {
          panic(err)
        }
        defer masterFile.Close()

        var lines []string
        scanner := bufio.NewScanner(masterFile)
        for scanner.Scan() {
          lines = append(lines, scanner.Text())
        }
        if err := scanner.Err(); err != nil {
          fmt.Fprintln(os.Stderr, err)
        }
				text := lines[len(lines)-1]
		    token := c.Publish("MCITOPIC", 0, false, "(H)" + text)
				token.Wait()

			case err := <-w1.Error:
				log.Fatalln(err)
			case <-w1.Closed:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-w2.Event:
        resourceFile, err := os.Open("Condition_1/" + resourceFileName)
        if err != nil {
          panic(err)
        }
        defer resourceFile.Close()

        var lines []string
        scanner := bufio.NewScanner(resourceFile)
        for scanner.Scan() {
          lines = append(lines, scanner.Text())
        }
        if err := scanner.Err(); err != nil {
          fmt.Fprintln(os.Stderr, err)
        }
				text := lines[len(lines)-1]
				textArray := strings.Fields(text)
		    token := c.Publish("MCITOPIC", 0, false, "(H)Tank A in range: " + textArray[8] + ", " + "Tank B in range: " + textArray[9])
				token.Wait()

			case err := <-w2.Error:
				log.Fatalln(err)
			case <-w2.Closed:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-w3.Event:
        trackingFile, err := os.Open("Condition_1/" + trackingFileName)
        if err != nil {
          panic(err)
        }
        defer trackingFile.Close()

        var lines []string
        scanner := bufio.NewScanner(trackingFile)
        for scanner.Scan() {
          lines = append(lines, scanner.Text())
        }
        if err := scanner.Err(); err != nil {
          fmt.Fprintln(os.Stderr, err)
        }
				text := lines[len(lines)-1]
				textArray := strings.Fields(text)
		    token := c.Publish("MCITOPIC", 0, false, "(H)Both in range: " + textArray[10])
				token.Wait()

			case err := <-w3.Error:
				log.Fatalln(err)
			case <-w3.Closed:
				return
			}
		}
	}()

	// Watch this folder for changes.
	if err := w1.Add("./" + masterFileName); err != nil {
		log.Fatalln(err)
	}

	// Watch test_folder recursively for changes.
	if err := w2.Add("./Condition_1/" + resourceFileName); err != nil {
		log.Fatalln(err)
	}

	if err := w3.Add("./Condition_1/" + trackingFileName); err != nil {
		log.Fatalln(err)
	}

	// Print a list of all of the files and folders currently
	// being watched and their paths.
	for path, f := range w1.WatchedFiles() {
		fmt.Printf("%s: %s\n", path, f.Name())
	}
	for path, f := range w2.WatchedFiles() {
		fmt.Printf("%s: %s\n", path, f.Name())
	}
	for path, f := range w3.WatchedFiles() {
		fmt.Printf("%s: %s\n", path, f.Name())
	}

	fmt.Println()

	// Trigger 2 events after watcher started.
	go func() {
		w1.Wait()
	}()
	go func() {
		w2.Wait()
	}()
	go func() {
		w3.Wait()
	}()

	// Start the watching process - it'll check for changes every 100ms.
	go func() {
		if err := w1.Start(time.Millisecond * 100); err != nil {
			log.Fatalln(err)
		}
	}()
	go func() {
		if err := w2.Start(time.Millisecond * 100); err != nil {
			log.Fatalln(err)
		}
	}()

	if err := w3.Start(time.Millisecond * 100); err != nil {
		log.Fatalln(err)
	}
}
