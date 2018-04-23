package main

import (
	"fmt"
	"log"
	"time"
  "bufio"
	"github.com/radovskyb/watcher"
  "os"
	"strings"
	"strconv"
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
	// w1.SetMaxEvents(50)
	// w2.SetMaxEvents(50)
	// w3.SetMaxEvents(50)

	// Only notify write events
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

	//***********************************************************************************************************//

  //Watcher for master resource file
	go func() {
		mapMaster := make(map[string]float64)
		mapMasterAttend := make(map[string]float64)
		for {
			select {
			case <-w1.Event:
				//fmt.Println(masterFileName + " this is the name")
        masterFile, err := os.Open(masterFileName)
        if err != nil {
          panic(err)
        }

        var lines []string
        scanner := bufio.NewScanner(masterFile)
        for scanner.Scan() {
          lines = append(lines, scanner.Text())
        }
        if err := scanner.Err(); err != nil {
          fmt.Fprintln(os.Stderr, err)
        }
				text := lines[len(lines)-1]

				masterFile.Close()

				textArray1 := strings.Split(text,"\t")
				timeing, _ := strconv.ParseFloat(textArray1[0], 64)

				//if triger fault happens add name of component with time to map
				if(textArray1[2] == "Script-triggered Fault") {
					mapMaster[textArray1[1]] = timeing
				} else {
					//else look through map and see if more than 5 seconds has commenced since action happended
					for k := range mapMaster {
    				if(timeing - mapMaster[k] > 5) {
							//if 5 seconds past add map to new map of attending actions and remove from first map in order to prevent sending multiple attend messages
							token := c.Publish("MCITOPIC", 0, false, "(H)Attend to: " + k)//send warning
							token.Wait()
							mapMasterAttend[k] = mapMaster[k]
							delete(mapMaster, k);
						}
						//sleep 200 milliseconds to prevent loss of messages that occur right after each other
						time.Sleep(200 * time.Millisecond)
					}
					temp := mapMaster[textArray1[1]]
					if(temp != 0) {
						//if user responded withing 5 seconds then remove from map
						if(textArray1[2] == "User Response"){
							delete(mapMaster, textArray1[1]);//remove from map
						}
					}
					temp2 := mapMasterAttend[textArray1[1]]
					if(temp2 != 0) {
						//if user responded after 5 seconds but before 10 seconds, notify user that it has been attended to
						if(textArray1[2] == "User Response"){
							delete(mapMasterAttend, textArray1[1]);//remove from map
							token := c.Publish("MCITOPIC", 0, false, "(H)User Responded: " + textArray1[1])
							token.Wait()//send acknoledgement
						}
						//if 10 seconds past a timeout will occur and the user will also be notified
						if(textArray1[2] == "Script-triggered Fault Timeout") {
							delete(mapMasterAttend, textArray1[1]);//remove from map
							token := c.Publish("MCITOPIC", 0, false, "(H)Timeout: " + textArray1[1])
							token.Wait()//send acknoledgement
						}
					}
				}
			case err := <-w1.Error:
				log.Fatalln(err)
			case <-w1.Closed:
				return
			}
		}
	}()

	//***********************************************************************************************************//
	//***********************************************************************************************************//

	//Watcher for resource, resource file
	go func() {
		mapResource := make(map[string]float64)
		mapResourceAttend := make(map[string]float64)
		for {
			select {
			case <-w2.Event:
        resourceFile, err := os.Open("Condition_1/" + resourceFileName)
        if err != nil {
          panic(err)
        }

        var lines []string
        scanner := bufio.NewScanner(resourceFile)
        for scanner.Scan() {
          lines = append(lines, scanner.Text())
        }
        if err := scanner.Err(); err != nil {
          fmt.Fprintln(os.Stderr, err)
        }
				text := lines[len(lines)-1]

				resourceFile.Close()
				//textArray2 := strings.Fields(text)
				textArray2 := strings.Split(text,"\t")
				timeing2, _ := strconv.ParseFloat(textArray2[0], 64)

				//if columns are zero add to map
				if(textArray2[8] == "0.000000") {
					//only log time if maps doesn't contain Tank A
					if(mapResource["Tank A"] == 0 && mapResourceAttend["Tank A"] == 0){
						mapResource["Tank A"] = timeing2
					}
				}

				if(textArray2[9] == "0.000000") {
					//only log time if maps doesn't contain Tank B
					if(mapResource["Tank B"] == 0 && mapResourceAttend["Tank B"] == 0){
						mapResource["Tank B"] = timeing2
					}
				}

				//Look through map and see if more than 5 seconds has commenced since action happended
				for k := range mapResource {
					if(timeing2 - mapResource[k] > 5) {
						//if 5 seconds past add map to new map of attending actions and remove from first map in order to prevent sending multiple attend messages
						token := c.Publish("MCITOPIC", 0, false, "(H)Attend to: " + k)//send warning
						token.Wait()
						mapResourceAttend[k] = mapResource[k]
						delete(mapResource, k);
					}
					//sleep 200 milliseconds to prevent loss of messages that occur right after each other
					time.Sleep(200 * time.Millisecond)
				}

				tempA := mapResource["Tank A"]
				if(tempA != 0) {
					//if user responded withing 5 seconds then remove from map
					if(textArray2[8] == "1.000000"){
						delete(mapResource, "Tank A");//remove from map
					}
				}

				tempB := mapResource["Tank B"]
				if(tempB != 0) {
					//if user responded withing 5 seconds then remove from map
					if(textArray2[9] == "1.000000"){
						delete(mapResource, "Tank B");//remove from map
					}
				}

				tempAttendA := mapResourceAttend["Tank A"]
				if(tempAttendA != 0) {
					//if user responded after 5, notify user that it has been attended to
					if(textArray2[8] == "1.000000"){
						delete(mapResourceAttend, "Tank A");//remove from map
						token := c.Publish("MCITOPIC", 0, false, "(H)User Responded: Tank A")
						token.Wait()//send acknoledgement
					}
				}

				tempAttendB := mapResourceAttend["Tank B"]
				if(tempAttendB != 0) {
					//if user responded after 5 seconds, notify user that it has been attended to
					if(textArray2[9] == "1.000000"){
						delete(mapResourceAttend, "Tank B");//remove from map
						token := c.Publish("MCITOPIC", 0, false, "(H)User Responded: Tank B")
						token.Wait()//send acknoledgement
					}
				}

			case err := <-w2.Error:
				log.Fatalln(err)
			case <-w2.Closed:
				return
			}
		}
	}()

	//***********************************************************************************************************//
	//***********************************************************************************************************//

	//Watcher for tracking, resource file
	go func() {
		mapTracking := make(map[string]float64)
		mapTrackingAttend := make(map[string]float64)
		for {
			select {
			case <-w3.Event:
        trackingFile, err := os.Open("Condition_1/" + trackingFileName)
        if err != nil {
          panic(err)
        }

        var lines []string
        scanner := bufio.NewScanner(trackingFile)
        for scanner.Scan() {
          lines = append(lines, scanner.Text())
        }
        if err := scanner.Err(); err != nil {
          fmt.Fprintln(os.Stderr, err)
        }
				text := lines[len(lines)-1]
				//textArray3 := strings.Fields(text)
				trackingFile.Close()
				textArray3 := strings.Split(text,"\t")
				timeing3, _ := strconv.ParseFloat(textArray3[0], 64)

				if(textArray3[10] == "0.000000") {
					if(mapTracking["Both out of Range"] == 0 && mapTrackingAttend["Both out of Range"] == 0){
						//Only add to map if Both out of range is not present
						mapTracking["Both out of Range"] = timeing3
					}
				}

				//Look through map and see if more than 5 seconds has commenced since action happended
				for k := range mapTracking {
					if(timeing3 - mapTracking[k] > 5) {
						//if 5 seconds past add map to new map of attending actions and remove from first map in order to prevent sending multiple attend messages
						token := c.Publish("MCITOPIC", 0, false, "(H)Attend to: " + k)//send warning
						token.Wait()
						mapTrackingAttend[k] = mapTracking[k]
						delete(mapTracking, k);
					}
					//sleep 200 milliseconds to prevent loss of messages that occur right after each other
					time.Sleep(200 * time.Millisecond)
				}

				tempBoth := mapTracking["Both out of Range"]
				if(tempBoth != 0) {
					//if user responded withing 5 seconds then remove from map
					if(textArray3[10] == "1.000000"){
						delete(mapTracking, "Both out of Range");//remove from map
					}
				}

				tempAttendBoth := mapTrackingAttend["Both out of Range"]
				if(tempAttendBoth != 0) {
					//if user responded after 5 seconds, notify user that it has been attended to
					if(textArray3[10] == "1.000000"){
						delete(mapTrackingAttend, "Both out of Range");//remove from map
						token := c.Publish("MCITOPIC", 0, false, "(H)User Responded: Both out of Range")
						token.Wait()//send acknoledgement
					}
				}
			case err := <-w3.Error:
				log.Fatalln(err)
			case <-w3.Closed:
				return
			}
		}
	}()

	//***********************************************************************************************************//

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

	go func() {
		for{
			masterFile, err := os.OpenFile(masterFileName, os.O_RDWR, 0644)
			if err != nil {
				panic(err)
			}
			_, _ = masterFile.WriteAt([]byte{' '}, 0) // Write at 0 beginning
			masterFile.Close()
			time.Sleep(5*time.Millisecond);

			trackingFile, err := os.OpenFile("Condition_1/" + trackingFileName, os.O_RDWR, 0644)
			if err != nil {
				panic(err)
			}

			_, _ = trackingFile.WriteAt([]byte{' '}, 0) // Write at 0 beginning
			trackingFile.Close()
			time.Sleep(5*time.Millisecond);

			resourceFile, err := os.OpenFile("Condition_1/" + resourceFileName, os.O_RDWR, 0644)
			if err != nil {
				panic(err)
			}

			_, _ = resourceFile.WriteAt([]byte{' '}, 0) // Write at 0 beginning
			resourceFile.Close()
			time.Sleep(5*time.Millisecond);
    }
	}()

	// Start the watching process - it'll check for changes every 100ms.
	go func() {
		if err := w1.Start(time.Millisecond * 5); err != nil {
			log.Fatalln(err)
		}
	}()
	go func() {
		if err := w2.Start(time.Millisecond * 10); err != nil {
			log.Fatalln(err)
		}
	}()

	if err := w3.Start(time.Millisecond * 10); err != nil {
		log.Fatalln(err)
	}
}
