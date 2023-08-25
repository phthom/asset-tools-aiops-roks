package main

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// AIO is a multipurpose program that help building solution with AIOps Manager (from CP4WAIOPs)
//
// - switch and login to several OpenShift clusters (20 max)
// - execute several commands concerning the AI manager : topology, metric, alert, story ...
//
// Philippe THOMAS
// aio program v340 -  GOLANG 19.4
//
// a JSON configuration file is used to read the configuration parameters to be used:  ~/.aio
// To create a first configuration file, use "aio -a" to add your first cluster
//
// Prerequisites CLIs: oc, ibmcloud(only if ROKS is used)
// 		for alerts you need a kafka connector (cp4waiops-cartridge-alerts-none-xxxxxx)
// 
//
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// Change List:
//
// - Support for Windows, Apple and Linux : gox
// 
//
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////


import (
	"context"
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"strconv"
	"path/filepath"
	"regexp"
	"sort"		
	"time"
	"encoding/base64"
	"crypto/x509"
	"io/ioutil"


	"github.com/manifoldco/promptui"						// prompt UI for CLI
	"github.com/tidwall/gjson" 								// GJSON - simple way to read a JSON value in a JSON payload
	"github.com/segmentio/kafka-go"							// Segmentio : Kafka-go witout dependies
	"github.com/segmentio/kafka-go/sasl/scram"				// Segmentio : Kafka-go SCRAM 
	"github.com/bitfield/script"							// pipeline script 
)

// Global Variables
///////////////////

var version = "340"

// print functions
var printf  = fmt.Printf
var println = fmt.Println
var sprintf = fmt.Sprintf

var verbose = true

var werbose = false // trace all debug functions
var route_obs = ""
var tokenC = ""
var tenant = "cfd95b7e-3bc7-4006-a4a8-a73a79c71255"

var iconp = "host"
var iconc = "host"
var jobid = "restTopology"
var ptags = `"hosts"`
var tagP = ptags
var tagC = ptags
var extN = ""

var req 		= ""
var call 		= ""
var callret 	= ""
var finalroute 	= ""
var parent 		= ""		// ID of parent resource
var child		= ""		// ID of child resource
var parentN 	= "" 		// Basic Name of a parent resource
var childN 		= ""  		// Basic Name of a child resource
var link 		= ""
var  countError uint
var  countTotal uint
var  countRes 	uint 		// total resources including edges
var projectN = ""
var currentC []byte

var kafkauser   = "cp4waiops-cartridge-kafka-auth-0"
var kafkapass 	= ""
var kafkatopic 	= ""
var broker 		= ""

var mapStore = make(map[string]string)	// Hash to store each resource definitions prior to print or send after transformation


//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// MAIN
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////


func main() {

	verbose = false

	roks		:= true
	title 		:= ""
	accountkey 	:= ""
	clusterID 	:= ""
	aimuser 	:= ""
	aimapikey 	:= ""
	ocpuser  	:= ""
	ocpkey	 	:= ""
	ocpurl 	 	:= ""
	obsrest 	:= ""
	topicalert 	:= ""

	////////////////////////
	// Actions
	////////////////////////
	loginAction 			:= false
	addParamAction 			:= false
	listParamAction 		:= false
	identityAction 			:= false
	metricAction 			:= false
	deleteMetricAction 		:= false
	countMetricAction 		:= false
	topoAction				:= false
	listAlertAction			:= false
	sendAlertAction			:= false
	postAlertAction			:= false
	findAlertAction			:= false
	deleteAlertAction		:= false
	quickAlertAction		:= false
	recentAlertAction		:= false
	listStoryAction			:= false
	listMetricResourceAction := false

	// action paramaters
	file  		:= ""
	fileN 		:= ""
	event       := ""
	text		:= ""

	////////////////////////
	// Structure JSON file
	///////////////////////

	type Entry struct {
		Title      	string `json:"title"`		// free text title
		ROKS		bool   `json:"roks"` 	  	// ROKS=true 
		Accountkey 	string `json:"accountkey"` 	// ROKS accountkey
		ClusterID  	string `json:"clusterID"`  	// ROKS clusterID 
		Aimuser  	string `json:"aimuser"`	  	// AIM user
		Aimapikey  	string `json:"aimapikey"`	// AIM API key
		OCPuser  	string `json:"ocpuser"`		// OCP user
		OCPkey 		string `json:"ocpkey"`		// OCP key
		OCPurl  	string `json:"ocpurl"`		// OCP url	
		Obsrest     string `json:"obsrest"`		// REST Observer JOBID
		Topicalert  string `json:"topicalert"`	// Kafka topic alert-none
		ProjectN    string `json:"projectN"`	// Namespace and project containing cp4waiops
	}

	var conf []Entry
	listT 	:= make([]string, 20)
	mapR	:= make(map[string]bool) // Hash for ROKS
	mapA 	:= make(map[string]string) 	// Hash for Account
	mapC 	:= make(map[string]string) 	// Hash for ClusterID
	mapU 	:= make(map[string]string)	// Hash for AIM user
	mapK 	:= make(map[string]string)	// Hash for AIM key
	mapX 	:= make(map[string]string) 	// Hash for OCP user
	mapY 	:= make(map[string]string)	// Hash for OCP key
	mapZ	:= make(map[string]string)	// Hash for OCP url
	mapO	:= make(map[string]string)	// Hash for REST Observer JOBID
	mapT	:= make(map[string]string)	// Hash for Kafka topic alert-none
	mapP    := make(map[string]string)	// Hash for Project Name

	//////////////////////
	// Reading Conf file
	/////////////////////

	dirname, _ 	:= os.UserHomeDir() // Home Directory
	configFile 	:= dirname + "/.aio"			// JSON config file for all the openshift clusters
	clusterFile := dirname + "/.aio.cluster" 	// Current cluster recently in use

	// Read config file
	fileContent, err := os.ReadFile(configFile)
	err = json.Unmarshal(fileContent, &conf)

	if err != nil {
		printfRed(".aio configuration file is empty or non-existing")
		addParamAction = true
	}

	currentC, _ = os.ReadFile(clusterFile) // Readinf the current cluster title
    
	//////////////////////
	// Argument Management
	//////////////////////

	args := len(os.Args)

	if args == 1 {
		loginAction = true
	}
	if args == 2 {
		cmd := os.Args[1]
		if cmd == "-v" { // Verbose
			loginAction = true
			verbose = true
		}
		if cmd == "-a" { // Add a line in .aio
			addParamAction = true
		}
		if cmd == "-l" { // List .aio
			listParamAction = true
		}
		if cmd == "-vl" { // List .aio
			listParamAction = true
			verbose = true
		}
		if (cmd == "-i")  || (cmd == "-c") { // identify current cluster
			identityAction = true
		}
		if (cmd == "alerts") || (cmd == "alert") || (cmd == "a") {
			listAlertAction = true
		}
		if (cmd == "stories") || (cmd == "story") || (cmd == "s") {
			listStoryAction = true
		}
		if (cmd == "-h")  || (cmd == "--help") { // Help
			printfCyan("AIO version "+version+" -- AIOps CLI")
			printfBlue("Multi-purpose Command Line for AI Manager")
			printfBlue("Use registered clusters described in ~/.aio")
			printfBlue("")
			printfBlue("Prereqs:")
			printfBlue("OpenSHift CLI: oc")
			printfBlue("IBM Cloud CLI: ibmcloud")
			printfBlue("These 2 commands can be downloaded from the OpenShift console interrogation mark icon (?)")
			printfBlue("Note1: Copy these 2 commands into a directory that is in your path")
			printfBlue("Note2: ibmcloud CLI is only mandatory if you are using Openshift on IBM Cloud or ROKS")
			printfBlue("Note3: For ROKS, you also need to add the KS plugin: ibmcloud plugin install ks ")
			printfBlue("")
			printfBlue("Usage:")
			printfBlue("aio       # select and login to a cluster")
			printfBlue("")
			printfBlue("aio -h,--help    # help mode")
			printfBlue("aio -v           # verbose mode -- can be also use in each subcommand like -vp or -vq ...")
			printfBlue("aio -a           # add a new cluster parameters stored in ~/.aio")
			printfBlue("aio -i,-c        # identify the current cluster")
			printfBlue("aio -l           # list all parameters in ~/.aio")
			printfBlue("")
			printfBlue("aio story -l                   # list all open stories")
			printfBlue("")
			printfBlue("aio alert -l                   # list all open alerts")
			printfBlue("aio alert -d                   # delete all alerts")
			printfBlue("aio alert -f text              # find alerts in the alert history")
			printfBlue("aio alert -p file.json         # post a specific set of alerts from a JSON file")
			printfBlue("aio alert -s '{}'              # send one alert in JSON format -- see below")
			printfBlue("aio alert -q 'resource,sev,summary' 	# send one quick alert now with a resource, a severity and summary")
			printfBlue("")
			printfBlue("IMPORTANT: you need to add a KAFKA event connector with cp4waiops-cartridge-alerts-none-xxxxx topic ")
			printfBlue("")
			printfBlue("Alert/Event JSON example:")
			printfBlue(`'{"sender": {"service": "Agent", "name": "monitor1","type": "App"},"resource": {"name": "vm1.com","hostname": "vm1","type": "IBM","ipaddress": "192.1.1.109","location": "Paris"},"type": {"classification": "HOST1.TEST.ABC","eventType": "problem"},"severity": 3,"summary": "explain 123 ABC","occurrenceTime": "2023-04-11T11:17:13.000000Z","expirySeconds": 0}'`)
			printfBlue("")
			printfBlue("aio topology -s                # Simulate/check access to Resource Manager(RM)" )
			printfBlue("aio topology -p test.csv       # PUT a CSV file (transformed into JSON) to Resource Manager" )
			printfBlue("aio topology -q test.csv       # POST a CSV file (transformed into JSON) to Resource Manager" )
			printfBlue("aio topology -d test.csv       # DELETE resources in Resource Manager from a CSV file" )
			printfBlue("aio topology -p test.json      # PUT a JSON file to Resource Manager" )
			printfBlue("aio topology -q test.json      # POST a JSON file to Resource Manager" )
			printfBlue("aio topology -d test.json      # DELETE resources in Resource Manager from a JSON file" )
			printfBlue("aio topology -j test.csv       # Transforms CSV file into a JSON file -- nothing sent to Resource Manager" )
			printfBlue("")
			printfBlue("IMPORTANT: you need to add a topology REST observer: restTopology")
			printfBlue("")
			printfBlue("Topology CSV example:")
			printfBlue("parent;icon;tags;link;child;icon;tags;")
			printfBlue(`AAA.1.22.333;iconA;"tagA","tagAB";bindsTo;BBB.1.22.333;iconB;"tagB","tagAB";`)
			printfBlue(`AAA.1.22.333;iconA;"tagA","tagAB";;;;;`)
			printfBlue("")
			printfBlue("Topology JSON example:")
			printfBlue(`{ "name": "AAA", "uniqueId": "AAA.1.22.333", "matchTokens": [ "AAA.1.22.333","AAA" ], "tags": [ "tagA","tagAB" ], "entityTypes": [ "iconA" ]}`)
			printfBlue(`{ "name": "BBB", "uniqueId": "BBB.1.22.333", "matchTokens": [ "BBB.1.22.333","BBB" ], "tags": [ "tagB","tagAB" ], "entityTypes": [ "iconB" ]}`)
			printfBlue(`{ "_fromUniqueId": "AAA.1.22.333", "_edgeType": "bindsTo", "_toUniqueId": "BBB.1.22.333"}`)
			printfBlue("")
			printfBlue("aio metric -c                  # List counts of metrics" )
			printfBlue("aio metric -r                  # List metric resources" )
			printfBlue("aio metric -d                  # Clear all metric values" )
			printfBlue("aio metric -p test.json        # POST a JSON file to the Metric Anomaly Detection" )
			printfBlue("")
			printfBlue("Metric JSON example:")
			printfBlue(`{"groups": [  {"timestamp": 1651300300000, "resourceID": "vm1", "metrics": {"CPU": 89}, "attributes": {"group": "APP", "node": "vm1"}}   ]}`)
			printfBlue("")
			bash("ibmcloud --help")
			bash("oc --help")
			printfGreen("CLIs: oc OK; ibmcloud OK")
			return
		}
	}
	if args == 3 { 			// 3 parameters : aio alerts -l
		cmd := os.Args[1]
		if (cmd == "alerts") || (cmd == "alert") || (cmd == "a") {
			switch os.Args[2] {
				case "-h": 
					printfBlue("aio alert -l                   # list all open alerts")
					printfBlue("aio alert -d                   # delete all alerts")
					printfBlue("aio alert -f text              # find alerts in the alert history")
					printfBlue("aio alert -p file.json         # post a specific set of alerts from a JSON file")
					printfBlue("aio alert -s '{}'              # send one alert in JSON format -- see below")
					printfBlue("aio alert -q 'resource,sev,summary' 	# send one quick alert now with a resource, a severity and summary")
					printfBlue("")
					printfBlue("IMPORTANT: you need to add a KAFKA event connector with cp4waiops-cartridge-alerts-none-xxxxx topic ")
					printfBlue("")
					printfBlue("Alert/Event JSON example:")
					printfBlue(`'{"sender": {"service": "Agent", "name": "monitor1","type": "App"},"resource": {"name": "vm1.com","hostname": "vm1","type": "IBM","ipaddress": "192.1.1.109","location": "Paris"},"type": {"classification": "HOST1.TEST.ABC","eventType": "problem"},"severity": 3,"summary": "explain 123 ABC","occurrenceTime": "2023-04-11T11:17:13.000000Z","expirySeconds": 0}'`)
					printfBlue("")
					return
				case "-l": 
					listAlertAction = true
				case "-vl": 
					listAlertAction = true	
					verbose = true
				case "-d": 
					deleteAlertAction = true
				case "-vd": 
					deleteAlertAction = true
					verbose = true	
				case "-r": 
					recentAlertAction = true
				case "-vr": 
					recentAlertAction = true
					verbose = true	
			}
		}
		if (cmd == "metrics") || (cmd == "metric") || (cmd == "m") {
			switch os.Args[2] {
				case "-h": 
					printfBlue("aio metric -c                  # List counts of metrics" )
					printfBlue("aio metric -r                  # List metric resources" )
					printfBlue("aio metric -d                  # Clear all metric values" )
					printfBlue("aio metric -p test.json        # POST a JSON file to the Metric Anomaly Detection" )
					printfBlue("")
					printfBlue("Metric JSON example:")
					printfBlue(`{"groups": [  {"timestamp": 1651300300000, "resourceID": "vm1", "metrics": {"CPU": 89}, "attributes": {"group": "APP", "node": "vm1"}}   ]}`)
					printfBlue("")
					return
				case "-d": 
					deleteMetricAction = true
				case "-vd": 
					deleteMetricAction = true	
					verbose = true
				case "-c": 
					countMetricAction = true
				case "-vc": 
					countMetricAction = true	
					verbose = true
				case "-r": 
					listMetricResourceAction = true
				case "-vr": 
					listMetricResourceAction = true	
					verbose = true
			}
		}
		if (cmd == "stories") || (cmd == "story") || (cmd == "s") {
			switch os.Args[2] {
				case "-h": 
					printfBlue("aio story -l                   # list all open stories")
					printfBlue("")
					return
				case "-l": 
					listStoryAction = true
				case "-vl": 
					listStoryAction = true	
					verbose = true
			}
		}
		if (cmd == "topology") || (cmd == "topo") || (cmd == "t") {
			switch os.Args[2] {
				case "-h": 
					printfBlue("aio topology -s                # Simulate/check access to Resource Manager(RM)" )
					printfBlue("aio topology -p test.csv       # PUT a CSV file (transformed into JSON) to Resource Manager" )
					printfBlue("aio topology -q test.csv       # POST a CSV file (transformed into JSON) to Resource Manager" )
					printfBlue("aio topology -d test.csv       # DELETE resources in Resource Manager from a CSV file" )
					printfBlue("aio topology -p test.json      # PUT a JSON file to Resource Manager" )
					printfBlue("aio topology -q test.json      # POST a JSON file to Resource Manager" )
					printfBlue("aio topology -d test.json      # DELETE resources in Resource Manager from a JSON file" )
					printfBlue("aio topology -j test.csv       # Transforms CSV file into a JSON file -- nothing sent to Resource Manager" )
					printfBlue("")
					printfBlue("IMPORTANT: you need to add a topology REST observer: restTopology")
					printfBlue("")
					printfBlue("Topology CSV example:")
					printfBlue("parent;icon;tags;link;child;icon;tags;")
					printfBlue(`AAA.1.22.333;iconA;"tagA","tagAB";bindsTo;BBB.1.22.333;iconB;"tagB","tagAB";`)
					printfBlue(`AAA.1.22.333;iconA;"tagA","tagAB";;;;;`)
					printfBlue("")
					printfBlue("Topology JSON example:")
					printfBlue(`{ "name": "AAA", "uniqueId": "AAA.1.22.333", "matchTokens": [ "AAA.1.22.333","AAA" ], "tags": [ "tagA","tagAB" ], "entityTypes": [ "iconA" ]}`)
					printfBlue(`{ "name": "BBB", "uniqueId": "BBB.1.22.333", "matchTokens": [ "BBB.1.22.333","BBB" ], "tags": [ "tagB","tagAB" ], "entityTypes": [ "iconB" ]}`)
					printfBlue(`{ "_fromUniqueId": "AAA.1.22.333", "_edgeType": "bindsTo", "_toUniqueId": "BBB.1.22.333"}`)
					printfBlue("")
					return
				case "-s": 
					topoAction = true
					req = "SIMU"
				case "-vs": 
					topoAction = true
					req = "SIMU"
					verbose = true	
			}
		}
	}
	if args == 4 { 			// 3 parameters 
		cmd := os.Args[1]
		if (cmd == "alerts") || (cmd == "alert") || (cmd == "a") {
			switch os.Args[2] {
				case "-f": 
					text = os.Args[3]
					findAlertAction = true
				case "-vf":
					text = os.Args[3]
					verbose = true
					findAlertAction = true
				case "-p": 
					file = os.Args[3]
					postAlertAction = true
				case "-vp":
					file = os.Args[3]
					verbose = true
					postAlertAction = true
				case "-s": 
					event = os.Args[3]
					sendAlertAction = true
				case "-vs":
					event = os.Args[3]
					verbose = true
					sendAlertAction = true
				case "-q": 
					event = os.Args[3]
					quickAlertAction = true
				case "-vq":
					event = os.Args[3]
					verbose = true
					quickAlertAction = true
			}
		}
		if (cmd == "metric") || (cmd == "metrics") || (cmd == "m") {
			switch os.Args[2] {
				case "-p": 
					file = os.Args[3]
					metricAction = true
				case "-vp":
					file = os.Args[3]
					verbose = true
					metricAction = true
			}
		}
		if (cmd == "topology") || (cmd == "topo") || (cmd == "t") {
			switch os.Args[2] {
				case "-p":
					fileN = os.Args[3]
					req = "PUT"
					topoAction = true
				case "-vp":	
					fileN = os.Args[3]
					req = "PUT"
					topoAction = true
					verbose = true
				case "-wp":	
					fileN = os.Args[3]
					req = "PUT"
					topoAction = true
					verbose = true
					werbose = true
				case "-q":
					fileN = os.Args[3]
					req = "POST"
					topoAction = true
				case "-vq":	
					fileN = os.Args[3]
					req = "POST"
					topoAction = true
					verbose = true
				case "-wq":	
					fileN = os.Args[3]
					req = "POST"
					topoAction = true
					verbose = true
					werbose = true
				case "-d":	
					fileN = os.Args[3]
					req = "DELETE"
					topoAction = true
				case "-vd":
					fileN = os.Args[3]
					req = "DELETE"
					topoAction = true
					verbose = true
				case "-wd":
					fileN = os.Args[3]
					req = "DELETE"
					topoAction = true
					verbose = true
					werbose = true
				case "-j":
					fileN = os.Args[3]
					req = "JSON"
					topoAction = true
					verbose = false			
			}
		}

	}

	///////////////////////////////////////
	///////////////////////////////////////
	// ACTIONS ACTIONS ACTIONS ACTIONS
	// ACTIONS ACTIONS ACTIONS ACTIONS
	///////////////////////////////////////
	///////////////////////////////////////



	///////////////////////////////////////
	// Add Parameters Action
	///////////////////////////////////////
	// aio -a
	// OK

	if addParamAction {
		reader := bufio.NewReader(os.Stdin)

		printf("Enter true if the cluster is running on IBM Cloud ROKS [true/false]: ")
		roksS, _ := reader.ReadString('\n')
		roksS = strings.TrimSuffix(roksS, "\n")
		roks, _ = strconv.ParseBool(roksS)

		printf("Enter a Title for the new cluster you want to login: ")
		title, _ = reader.ReadString('\n')
		title = strings.TrimSuffix(title, "\n")

		printf("Enter the project name for cp4waiops [cp4waiops]: ")
		projectN, _ = reader.ReadString('\n')
		projectN = strings.TrimSuffix(projectN, "\n")
		if projectN == "" {
			projectN = "cp4waiops"
		}

		if roks {
			printf("Enter the account key - IBM Cloud Console > target account > Manage > IAM > API key > Create: ")
			accountkey, _ = reader.ReadString('\n')
			accountkey = strings.TrimSuffix(accountkey, "\n")

			printf("Enter the ClusterID - IBM Cloud Console > target account > OpenShift Clusters > Select Cluster > ClusterID: ")
			clusterID, _ = reader.ReadString('\n')
			clusterID = strings.TrimSuffix(clusterID, "\n")
		}

		if !roks {
			printf("Enter the OCP user [kubeadmin]: ")
			ocpuser, _ = reader.ReadString('\n')
			ocpuser = strings.TrimSuffix(ocpuser, "\n")
			if ocpuser == "" {
				ocpuser = "kubeadmin"
			}

			printf("Enter the OCP key or password: ")
			ocpkey, _ = reader.ReadString('\n')
			ocpkey = strings.TrimSuffix(ocpkey, "\n")

			printf("Enter the OCP API url: ")
			ocpurl, _ = reader.ReadString('\n')
			ocpurl = strings.TrimSuffix(ocpurl, "\n")
		}
		

		printf("Enter the AIM user - from CP4WAIOPS console avatar [admin]: ")
		aimuser, _ = reader.ReadString('\n')
		aimuser = strings.TrimSuffix(aimuser, "\n")
		if aimuser == "" {
			aimuser = "admin"
		}

		printf("Enter the AIM API key - from CP4WAIOPS console avatar: ")
		aimapikey, _ = reader.ReadString('\n')
		aimapikey = strings.TrimSuffix(aimapikey, "\n")

		printf("Enter REST Observer JobID (in CP4AIOps) that is used for ingesting topologies [restTopology]:  ")
		obsrest, _ = reader.ReadString('\n')
		obsrest = strings.TrimSuffix(obsrest, "\n")
		if obsrest == "" {
			obsrest = "restTopology"
		}

		printf("Enter Kafka topic (in CP4AIOps) that is used for ingesting alerts [none] :  ")
		topicalert,_ = reader.ReadString('\n')
		topicalert = strings.TrimSuffix(topicalert, "\n")
		if obsrest == "" {
			obsrest = "none"
		}

		// End of questions

		obj := Entry{Title: title, ROKS: roks, Accountkey: accountkey, ClusterID: clusterID, Aimuser: aimuser, Aimapikey: aimapikey, OCPuser: ocpuser, OCPkey: ocpkey, OCPurl: ocpurl, Obsrest: obsrest, Topicalert: topicalert, ProjectN: projectN }
		conf = append(conf, obj)
		jsoncode, _ := json.MarshalIndent(conf, "", " ")
		os.WriteFile(configFile, jsoncode, 0644)
	}

	// Load config file into an ARRAY
	i := 0
	for _, val := range conf {
		listT[i] = val.Title
		mapR[val.Title] = val.ROKS
		mapA[val.Title] = val.Accountkey
		mapC[val.Title] = val.ClusterID
		mapU[val.Title] = val.Aimuser
		mapK[val.Title] = val.Aimapikey
		mapX[val.Title] = val.OCPuser
		mapY[val.Title] = val.OCPkey
		mapZ[val.Title] = val.OCPurl
		mapO[val.Title] = val.Obsrest
		mapT[val.Title] = val.Topicalert
		mapP[val.Title] = val.ProjectN
		i++
	}
	
	////////////////////////////////////////////
	// List configuration Action
	////////////////////////////////////////////
	// aio -l
	// OK

	if listParamAction {
		for i := 0; i < 20; i++ {
		 title = listT[i]
		 if title == "" {
			break
		 }
		 roks = mapR[title]
		 accountkey = mapA[title]
		 clusterID = mapC[title]
		 aimuser = mapU[title]
		 aimapikey = mapK[title]
		 ocpuser = mapX[title]
		 ocpkey = mapY[title]
		 ocpurl = mapZ[title]
		 obsrest = mapO[title]
		 topicalert = mapT[title]
		 projectN = mapT[title]
		}
		out, _ := script.File(configFile).String()	// cat param file
		printf("\r    \r"+out+"\n")
		return
	}

	////////////////////////////////////////////
	// Identity Action
	////////////////////////////////////////////
	// aio -c/-i
	// OK

	if identityAction {
		project := "oc project"
		show, _ := bash(project)
		println("\r" + string(currentC) +" | "+ show)
		project = "oc project " + projectN
		bash(project)
		return
	}


	////////////////////////////////////////////
	// login Action
	///////////////////////////////////////////
	// aio 
	// OK

	if loginAction {
			prompt := promptui.Select{
				Label: "Login to a Cluster",
				Items: listT,
			}

			_, result, err := prompt.Run()
			checkErr(err)

			// printf("You choose %q\n", result)
			debug("ClusterID: "+mapC[result])
			title 		= result
			roks 		= mapR[title]
			accountkey 	= mapA[title]
			clusterID 	= mapC[title]
			aimuser 	= mapU[title]
			aimapikey 	= mapK[title]
			ocpuser 	= mapX[title]
			ocpkey 		= mapY[title]
			ocpurl 		= mapZ[title]
			obsrest 	= mapO[title]
			topicalert 	= mapT[title]
			projectN 	= mapP[title]
			printf("\rIn progress .")

			// Login to ROKS 
			if roks { 
				login := "ibmcloud login --apikey " + accountkey + " -r eu-gb"
				bash(login)
				debug( "\rIBMCloud Login OK      ")
			
				ocp := "ibmcloud oc cluster config -c " + clusterID
				bash(ocp)
				debug( "Config Cluster OK        ")
			
				oclog := "oc login -u apikey -p " + accountkey
				bash(oclog)
				printfGreen("\rROKS Login OK     ")
			
			// Login to OCP
			} else {
				login := "oc login "+ocpurl+" -u "+ocpuser+" -p "+ocpkey+" --insecure-skip-tls-verify=true"
				bash(login)
				printfGreen("\rOCP Login OK     ")
			
			}

			// Assign values
			if obsrest == "" {
				jobid = "restTopology"
				obsrest = "restTopology"
			} else {
				 jobid = obsrest 
			}
			kafkatopic = topicalert			

			// current cluster
			currentC = []byte(title)
			os.WriteFile(clusterFile, currentC, 0644)

			project := "oc project " + projectN
			show, _ := bash(project)
			printfGreen("\r" + string(currentC) +" | "+ show)
			return

	}

	////////////////////////////////////////////
	// Metrics Action
	///////////////////////////////////////////
	// aio metrics -p file.json
	// OK

	if metricAction {

		// Reading JSON file
		debug("Reading JSON file to be sent")
		jsonData, err := script.File(file).String()	// cat param file
		if err != nil {
			printfRed("ERROR Reading JSON File")
			panic(err)
		}
		debug( "JSON file OK")

		// Login to OpenShift Cluster
		project := "oc project " + projectN
		show, _ := bash(project)
		debug("\r" + string(currentC) +" | "+ show)

		// Get the route
		debug("Getting the Metric Route ...")
		rout,_ := bash(`oc get route | grep ibm-nginx-svc | awk '{print $2}'`)
		rout = strings.TrimSuffix(string(rout), "\n")
		debug( rout)
		debug( "Route OK")

		// Get AIM API KEY
		currCluster := strings.Split(string(currentC), "|")
		title = strings.TrimSpace(currCluster[0]) // get the title of the current clster
		aimuser = mapU[title] // get AIM user
		aimapikey = mapK[title] // get AIM API key for the previous user
		debug( string(aimuser)+ "  |  " + string(aimapikey))
		debug( "AIM api key OK")

		// Token
		debug("Getting the token")
		token1 := `echo ` + aimuser + `:` + aimapikey + ` | base64`
		token, _ := bash(token1)
		token = strings.TrimSuffix(string(token), "\n")
		debug( token)
		debug( "Token collected")

		// Initialize transports and http client
		debug("Initialization of routes and http client")
		endpoint := "/aiops/api/app/metric-api/v1/metrics"
		finalroute := "https://" + rout + endpoint
		debug(finalroute)

		// Sending the data
		println("\rSending " +file+ " to Metric Anomaly Detection with POST Request")
		status,data := httpSend("POST", finalroute, "ZenApiKey " + token, jobid, jsonData)
		println("\r"+data+"              ")
		printlnGR(status)

		return
	}

	////////////////////////////////////////////
	// Count Metric Action
	///////////////////////////////////////////
	// aio metrics -c
	// OK


	if countMetricAction {
		casspass := CassAccess() // Get cassandra password
		println("\rListing Metric Counts")
		showMR, _ := bash(`oc exec -it  aiops-topology-cassandra-0  -- sh -c 'cqlsh -u admin -p `+casspass+` --ssl -e "SELECT count(*) FROM tararam.md_metric_resource;"' `)
		debug("Metric resources: " + showMR)	
		rcTabMR := strings.Split(string(showMR), "\n")
		rc5MR := strings.TrimSpace(rcTabMR[4]) 
		printf("\rMetric Resources: %-12s\n",rc5MR)
		return
	}
	
	////////////////////////////////////////////
	// List Metric Resource Action
	///////////////////////////////////////////
	// aio metrics -r
	// OK

	if listMetricResourceAction {
		casspass := CassAccess() // Get cassandra password
		println("\rListing Metric Resources")
		showMR, _ := bash(`oc exec -it  aiops-topology-cassandra-0 -- sh -c 'cqlsh -u admin -p `+casspass+` --ssl -e "SELECT metric_name,resource_name,group_name FROM tararam.md_metric_resource;"' `)
		script.Echo(showMR).Reject("Unable to use a TTY").Stdout() // Display all lines from cassandra DB concerning the metric resources
		return
	}

	////////////////////////////////////////////
	// Delete Metric Action
	///////////////////////////////////////////
	// aio metrics -d
	// OK

	if deleteMetricAction {
		casspass := CassAccess() // Get cassandra password
		println("\rSuppressing all Metrics")
		
		CheckOK(`oc scale deploy aiops-ir-analytics-metric-action aiops-ir-analytics-metric-api aiops-ir-analytics-metric-spark --replicas=0`, `oc get pods |grep "\-metric\-" |wc -l`, "0")

		show, _ := bash(`oc exec -it  aiops-topology-cassandra-0  -- sh -c 'cqlsh -u admin -p `+casspass+` --ssl -e "TRUNCATE  tararam.dt_metric_value;"' `)
		debug("Truncate values: " + show)
		show, _ = bash(`oc exec -it  aiops-topology-cassandra-0  -- sh -c 'cqlsh -u admin -p `+casspass+` --ssl -e "TRUNCATE  tararam.md_metric_resource;"' `)
		debug("Truncate resources: " + show)			
		printf("\rTruncate Value and resource tables\n")

		CheckOK(`oc scale deploy aiops-ir-analytics-metric-action aiops-ir-analytics-metric-api aiops-ir-analytics-metric-spark --replicas=1`, `oc get pods |grep "\-metric\-" |grep '1/1' |wc -l` , "3")

		printfGreen("\rAll metric values were deleted\n")
		return
	}
	
	////////////////////////////////////////////
	// Topology Action
	///////////////////////////////////////////
	// aio topology
	// OK

	if topoAction {
		printlnj("\rSending TOPOLOGY with "+req+" request to Observer-JobID " + jobid)

		// Getting data file extension
		extN = strings.ToLower(filepath.Ext(fileN))

		// Login to OpenShift Cluster
		project := "oc project " + projectN
		show, _ := bash(project)
		debug("\r" + string(currentC) +" | "+ show)

		// Get the cpd route
		rout, err := bash(`oc get route | grep ibm-nginx-svc | awk '{print $2}'`)
		route_cpd := strings.TrimSuffix(string(rout), "\n")
		debug(route_cpd)

		// Get AIM API KEY
		currCluster := strings.Split(string(currentC), "|")
		title = strings.TrimSpace(currCluster[0]) // get the title of the current clster
		aimuser = mapU[title] // get AIM user
		aimapikey = mapK[title] // get AIM API key for the previous user
		debug( string(aimuser)+ "  |  " + string(aimapikey))

		// Initialize transports
		endpoint := "/icp4d-api/v1/authorize"
		finalroute := "https://" + route_cpd + endpoint
		
		var jsonData = []byte(`{
			"username": "`+aimuser+`", 
			"api_key": "`+ aimapikey +`" 
		}`)

		// Building User Token
		debug("Getting the User token")
		token1 := `echo ` + aimuser + `:` + aimapikey + ` | base64`
		token, _ := bash(token1)
		token = strings.TrimSuffix(string(token), "\n")
		debug( token)
		debug( "User token is fine")
		
		// http send to API
		status,data := httpSend("POST", finalroute, "Bearer "+token,"",string(jsonData[:]))
		debug("\r"+data)
		debug(status)
		
		// Get the topo token from the resulting HHTP request
		tokenC = gjson.Get(data, "token").String()
		tokenC = strings.TrimSuffix(tokenC, "\n")
		
		// Get Observer Route
		route_obs_cmd := `oc get routes -l release=aiops-topology -o jsonpath='{range .items[*]}https://{.spec.host}{.spec.path}{"\n"}{end}' | grep rest-observer`
		route_obs, err = bash(route_obs_cmd)
		if err != nil {
			printfRed("ERROR rest-observer 'restTopology' route is not active -- add global.enableAllRoutes: true")
			panic(err)
		}

		route_obs = strings.TrimSuffix(route_obs, "\n")
		debug(route_obs)

		// Checking AIM Topology Manager
		if req == "SIMU" {
			printf( "\rChecking Topology Info-Service")
			status,data := httpSend("GET", route_obs+"/service/info", "Bearer "+tokenC, jobid,"")
			println("\r"+data)
			printlnGR(status)
			return
		}

		// Different processing (JSON or CSV)
		
		if extN == ".csv" {
			script.File(fileN).FilterLine(CSVprocessing).CountLines()
		} else if extN == ".json" {
			script.File(fileN).FilterLine(JSONprocessing).CountLines()
		}	
		
		count, max := FINALprocessing()

		if req != "JSON" {
			printf("\r%d/%d Resources have been processed\n",count, max)
		}
		
		return
	}

	//////////////////////
	// List Story Action
	/////////////////////
	// aio stories -l
	// OK


	if listStoryAction {		//list Stories
		addRouteDatalayer() 
		userpass, urlDatalayer := DataLayerAccess()
		urlStories := `https://`+urlDatalayer+`/irdatalayer.aiops.io/active/v1/stories`
		debug ("\n"+ urlStories)
		
		status, jsonStories := httpSend("GET", urlStories,"Basic "+userpass, "", "")
		debug(jsonStories)
		debug(status)
		
		dst := &bytes.Buffer{}
		if err := json.Compact(dst, []byte(jsonStories)); err != nil {							// Compact all lines to one line
			panic(err)
		}
		printStories := dst.String()
		
		// Extract Stories from JSON results	
		printStories = strings.TrimPrefix(printStories, `{"stories":[`)							// remove start of json
		printStories = strings.TrimSuffix(printStories, "]}")									// remove end of json
		printStories = strings.Replace(printStories, `]},{"id":`, `]},`+"\n"+`{"id":`, -1) 		// add newline between new id
		debug("\r"+printStories)

		println("\rShowing active stories")
		script.Echo(printStories).FilterLine(extractStories).Stdout()							// extract story information and print
		
		return
	}


	//////////////////////
	// List Alerts Action
	/////////////////////
	// aio alerts -l
	// OK

	if listAlertAction {		//list Alerts
		// Get the alerts
		casspass := CassAccess() 
		println("\rShowing active alerts")
		printalerts := `oc exec -it  aiops-topology-cassandra-0  -- sh -c 'cqlsh -u admin -p ` +casspass+ ` --ssl -e "SELECT payload FROM aiops.alerts;"' | grep '"state":"open"' | sed 's/.*1;33m//' | sed 's/\[0m.//' | sed 's/^\ *//g' `
		listalerts, _ := bash(printalerts)
		debug(listalerts+"\n")
		
		// extract story information and print
		printf("\r")
		script.Echo(listalerts).Reject("Unable to use a TTY").FilterLine(extractAlerts).Stdout()			
		return

	}

	////////////////////////////
	// Post a file of Alerts Action 
	///////////////////////////
	// aio alert -p file.json 
	// OK

	if postAlertAction { // send a alerts json file to AI Manager 
		kafkapass, kafkatopic, broker = kafkaAccess() // Get kafka password and topic
		println("\rSending Alerts on Topic: "+ kafkatopic)
		script.File(file).FilterLine(sendAlert).Stdout()
		return
	}

	////////////////////////
	// Send one Alert Action
	///////////////////////
	// aio alert -s '{}' 
	// OK

	if sendAlertAction { // Send 1 event/alert as parameter
		kafkapass, kafkatopic, broker = kafkaAccess() // Get kafka password and topic
		println("\rSending Alert on Topic " +kafkatopic)
		script.Echo(event).FilterLine(sendAlert).Stdout()
		return
	}

	//////////////////////////
	// Find Alerts Action
	/////////////////////////
	// aio alerts -f text
	// Dependencies : oc		OK

	if findAlertAction { // Search alerts in alert history in Cassandra
		// find the alerts
		casspass := CassAccess() 
		printalerts := `oc exec -it  aiops-topology-cassandra-0  -- sh -c 'cqlsh -u admin -p ` +casspass+ ` --ssl -e "SELECT payload FROM aiops.alerts;"' | grep '` +text+ `' | sed 's/.*1;33m//' | sed 's/\[0m.//' | sed 's/^\ *//g' `
		listalerts, _ := bash(printalerts)
		debug(listalerts+"\n")

		// extract story information and print
		println("\rFinding alerts in database with "+text)
		script.Echo(listalerts).Reject("Unable to use a TTY").FilterLine(extractAlerts).Stdout()			
		return

	}

	//////////////////////////
	// Delete Alerts Action
	/////////////////////////
	// aio alerts -d
	// Dependencies : OC          		OK

	if deleteAlertAction { // Delete all open alerts
		addRouteDatalayer() 
		userpass, urlDatalayer := DataLayerAccess()
		urlAlerts := `https://`+urlDatalayer+`/irdatalayer.aiops.io/active/v1/alerts`
		debug ("\n"+ urlAlerts)

		alertdel := `{"state": "closed"}`
		status, data := httpSend("PATCH", urlAlerts,"Basic "+userpass, "", alertdel)
		println("\rDeleting all alerts")
		
		affected := gjson.Get(data, "affected")
		printf("\r                                             ")
		println("\rAlerts closed: "+affected.String())
		printlnGR(status)

		return
	}

	//////////////////////////
	// quick  Alerts Action
	/////////////////////////
	// aio alerts -q 'resource, severity, summary'
	// OK

	if quickAlertAction { // quick add a new alert
		addRouteDatalayer() 
		userpass, urlDatalayer := DataLayerAccess()
		urlAlerts := `https://`+urlDatalayer+`/irdatalayer.aiops.io/active/v1/events`
		debug ("\n"+ urlAlerts)
		
		resource, after, _   := strings.Cut(event,",") // split string to get resource
		severity, summary,_  := strings.Cut(after,",") // split string to get severity and summary
		currentTime := time.Now()
		debug(resource+"|"+ severity+"|"+summary)
		TS:=sprintf(currentTime.Format("2006-01-02T15:04:05"))+".000000Z"
		debug(TS)

		printf("\rQuikly send one alert\n")
		alertdef :=`{"sender": {"service": "Agent", "name": "Monitor","type": "Application"},"resource": {"name": "`+resource+`","hostname": "`+resource+`","type": "IBM","ipaddress": "192.1.1.109","location": "Paris"},"type": {"classification": "`+resource+`.Application.Monitor","eventType": "problem"},"severity": `+severity+`,"summary": "`+summary+`","occurrenceTime": "`+TS+`","expirySeconds": 0}` 

		println(alertdef)
		status, data := httpSend("POST", urlAlerts,"Basic "+userpass, "", alertdef)
		printf("\r                                                               ")
		println("\r"+data)
		printlnGR(status)

		return
	}

	//////////////////////////
	// Recent Alerts Action
	/////////////////////////
	// aio alerts -r
	// OK

	if recentAlertAction { // list all open alerts
		addRouteDatalayer() 
		userpass, urlDatalayer := DataLayerAccess()
		urlAlerts := `https://`+urlDatalayer+`/irdatalayer.aiops.io/active/v1/alerts`
		debug ("\n"+ urlAlerts)

		status, jsonAlerts := httpSend("GET", urlAlerts,"Basic "+userpass, "", "")
		debug(jsonAlerts)
		debug(status)
		
		println("\rShowing recent alerts")
		dst := &bytes.Buffer{}
		if err := json.Compact(dst, []byte(jsonAlerts)); err != nil {					// Compact all lines to one line
			panic(err)
		}
		listAlerts := dst.String()


		// Extract Alerts from JSON results
		listAlerts,_ = script.Echo(listAlerts).Join().String()											// Join all lines in one	
		listAlerts = strings.TrimPrefix(listAlerts, `{"alerts":[`)										// remove start of json
		listAlerts = strings.TrimSuffix(listAlerts, ` ]}`)												// remove end of json
		listAlerts = strings.Replace(listAlerts, `,{"sender":`, "\n"+`,{"sender":`, -1) 				// add newline between new ids
		
		// printf(listalerts+"\n")
		debug(listAlerts+"\n")
		printf("\r")
		script.Echo(listAlerts).FilterLine(extractAlerts).Stdout()			
		return
	}

printf("\rAIO is a CLI tool for AI manager\n")
printf("\rUse 'aio --help' to get all the different options.\n")

}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// END OF PROGRAM
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////




/////////////////////////
// Small Functions
/////////////////////////

// return bash command results
// OK
func bash(s string) (string, error) {
	
	cmd := exec.Command("bash", "-c", s)
	o, err := cmd.CombinedOutput()
	out := string(o)
	out = strings.TrimSuffix(out, "\n")
	if err != nil {
		printfRed("\rERROR " + s)
		log.Fatal(err)
	}
	return out, err
}

// Colored print
func printfBlue(s string)  	{ printf("\033[1;34m%s\033[0m\n",s) }
func printfGreen(s string) 	{ printf("\033[1;32m%s\033[0m\n",s) }
func printfYellow(s string) { printf("\033[1;33m%s\033[0m\n",s) }
func printfRed(s string) 	{ printf("\033[1;31m%s\033[0m\n",s) }
func printfCyan(s string)	{ printf("\033[0;36m%s\033[0m\n",s) }

// Colored println depending on HTTP code
func printlnGR(s string) {
	if strings.Contains(s, "20") {
		printfGreen("\r"+s)
	} else {
		printfRed("\r"+s)
	}
}

// Verbose debug print
func debug(s string) {
	
	if req == "JSON" { 	// avoid printing
		return
	}
	
	if verbose {
		printfCyan(s)
	} else {
		printf(".")
	}
}

// checking error and exit
func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// Used in topo -j to display only the json code
func printlnj(s string) {
	if req == "JSON" {
		return
	}
	println(s)
}

// Used for topology names/ID
func purify(s string) string {													// A revoir
	s = regexp.MustCompile(`[^a-zA-Z0-9.-]+`).ReplaceAllString(s, "")
	return s
}

// Write to a YAML file
func write(s string) {
	script.Echo(s+"\n").AppendFile("addroute.yaml")
}


/////////////////////////
// BIG Functions
/////////////////////////


// Processing each CSV line into JSON in topo
// OK
func CSVprocessing(line string) string {

	line = strings.TrimSuffix(line, "\n")			// remove \n
	data := strings.Split(line, ";") 				// create a data slice
	
	dataL := len(data)								// length of the data slice
	if dataL < 3 {
		printf("Error on this line: too few parameters"+ line) 			// Parent definition is incomplete
		return ""
	}


	// Parent management

	parent = strings.TrimSpace(data[0]) 			// parent ID
	parentA := strings.Split(parent, ".") 			// Split parent full name with .
	parentN = parentA[0]							// Get the first name to be parent Name
	
	iconp = strings.TrimSpace(data[1]) 				// parent icon
	if iconp == "" {
		iconp = "host"
	} 
	
	tagP = data[2]
	tagP1 := strings.Count(tagP, `"`)
	if tagP1 < 1 {
		tagP = `"`+tagP+`"`
	}
	if tagP == "" {									// tags could be "tagA","tagB","tagC"
		tagP = ptags								// 
	} 				

	parent_json :=  `{ "name": "`+ parentN + `"` 
	parent_json +=  `, "uniqueId": "` + parent  + `"`
	parent_json +=  `, "matchTokens": [ "` +parent+ `","`+parentN+ `" ]`
	parent_json +=  `, "tags": [ ` + tagP  + ` ]`
	parent_json +=  `, "entityTypes": [ "` + iconp + `" ]`
	parent_json +=  `}`

	debug( parent_json)
	store(parent, parent_json, "resources")

	if dataL < 7 {
		return ""										// only have a parent (no link, no children)
	}

	// Link edge management

	link = data[3] 									// edge link between parent and child


	// Children Management

	child = strings.TrimSpace(data[4]) 				// child ID
	childA := strings.Split(child, ".") 			// Split child full name with .
	childN = childA[0]								// Get the first name to be child Name
	
	iconc = strings.TrimSpace(data[5]) 				// child icon
	if iconc == "" {
		iconc = "host"
	} 
	
	tagC = data[6]
	tagC1 := strings.Count(tagC, `"`)
	if tagC1 < 1 {
		tagC = `"`+tagC+`"`
	}
	if tagC == "" {									// tags could be "tagA","tagB","tagC"
		tagC = ptags								// Default tag if empty
	} 					

	child_json :=  `{ "name": "`+ childN + `"` 
	child_json +=  `, "uniqueId": "` + child  + `"`
	child_json +=  `, "matchTokens": [ "` +child+ `","`+childN+ `" ]`
	child_json +=  `, "tags": [ ` + tagC  + ` ]`
	child_json +=  `, "entityTypes": [ "` + iconc + `" ]`
	child_json +=  `}`

	debug( child_json)

	link_json :=  `{ "_fromUniqueId": "`+ parent + `"` 
	link_json +=  `, "_edgeType": "` + link  + `"`
	link_json +=  `, "_toUniqueId": "`+ child  + `"`
	link_json +=  `}`
	
	edge := parent+">"+link+">"+child

	debug( link_json)
	
	store(child, child_json, "resources")
	store(edge, link_json, "references")

	return ""
}

// REST request to send topo data to RM
// OK
func REST(res, jsonN, typeN string) string {

    finalroute = ""
	req = purify(req)
	rt := "000"

	switch req {

		case "POST": 														// POST request
				finalroute = route_obs + "/rest/" + typeN
				debug(finalroute)
				status,_ := httpSend(req, finalroute, "Bearer "+tokenC, jobid, jsonN)
				println("\r"+jsonN)
				printlnGR(status)
				rt = status
				
		case "PUT": 	
				finalroute = route_obs+ "/rest/" + typeN			// PUT Request										
				if typeN=="references" {
					req = "POST"
				}
				debug(finalroute)
				status,_ := httpSend(req, finalroute, "Bearer "+tokenC, jobid, jsonN)
				println("\r"+jsonN)
				printlnGR(status)
				rt = status
				
		case "DELETE": 														// DELETE request
				if typeN=="references" {
					return "200"
				}
				finalroute = route_obs + "/rest/" + typeN + "/" + res
				debug(finalroute)
				status,_ := httpSend(req, finalroute, "Bearer "+tokenC, jobid, "")
				println("\r"+jsonN)
				printlnGR(status)
				rt = status
				
		case "JSON": 															// Generate JSON to standard output
				println(jsonN) 												// No comma necessary between json items
				rt = "000"
	}
	return rt
}

// store all resources(vertex) and edges in a hash table mapStore
// OK
func store(res, jsonN, typeN string) {
	if res == "" {
		return
	}
	key := typeN+":"+res 		// key is a combination of type and resource name
	_, ok := mapStore[key]
	if !ok {
		mapStore[key] = jsonN
		countRes++
	}

}

// JSON Processing
// OK
func JSONprocessing(line string) string {	
	if strings.Contains(line, "_toUniqueId") == true { // edge
		FID := gjson.Get(line, "_fromUniqueId")
		FIDv := FID.String()
		edge := gjson.Get(line, "_edgeType")
		edgev := edge.String()
		TID := gjson.Get(line, "_toUniqueId")
		TIDv := TID.String()
		store(FIDv+">"+edgev+">"+TIDv, line, "references")
	} else {
		UID := gjson.Get(line, "uniqueId")
		UIDv := UID.String()  		// resource		
		store(UIDv, line, "resources")
	}
	return line
}


// Print request without full token (20 characters)
func smallToken(c string) {
	before,after,_ := strings.Cut(c,"Bearer ") // split string from the call request c
	req := before + "Bearer $TOKEN"
	_,after1,_ := strings.Cut(after,`" `) // split string after the token
	req += `" ` + after1
	debug(req) 
}

// Topology final processing
// OK
func FINALprocessing() (int, int) {	// Read mapStore and send REST API

	keys := make([]string, 0, len(mapStore))
	count := 0
	max := 0

	for k := range mapStore {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, k := range keys {
		Klist := strings.Split(k, ":")
		rt := REST(Klist[1], mapStore[k], Klist[0])
		max ++
		if strings.Contains(rt, "20") {
			count ++
		}
	}
    return count, max
}

// Access to cassandra DB and return password
// OK 
func CassAccess() (string) { 
	project := "oc project " + projectN
	show, _ := bash(project)
	debug("\r" + string(currentC) +" | "+ show)
	casspass, _ := bash(`oc get secret aiops-topology-cassandra-auth-secret -o jsonpath --template '{.data.password}' | base64 --decode`)
	casspass = strings.TrimSuffix(casspass, "\n")
	debug("Cassandra Secret: " + casspass)
	return casspass
}



// Return KAFKA password, topic, broker for later access
// OK
func kafkaAccess() (string, string, string) {
	project := "oc project " + projectN
	show,_ := bash(project)
	debug("\r" + string(currentC) +" | "+ show)
	bash(`oc extract secret/iaf-system-cluster-ca-cert --keys=ca.crt --to=-> ca.crt`)
	broker,_   := bash(`oc get route iaf-system-kafka-bootstrap -o jsonpath='{.spec.host}' `)
	broker = broker+":443"
	password,_ := bash(`oc get secret cp4waiops-cartridge-kafka-auth-0 --template={{.data.password}} | base64 --decode`) 

	mechanism, err := scram.Mechanism(scram.SHA512, kafkauser, password)
	if err != nil {
		panic(err)
	}
	cert, err := ioutil.ReadFile("./ca.crt")
    if err != nil {
        log.Fatal(err)
    }

	ctx := context.Background()
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cert)

	conf := &tls.Config{
		RootCAs: certPool,
	}

	dialer := &kafka.Dialer{
		SASLMechanism: mechanism,
		TLS:  conf,
	  }
	
	conn, err := dialer.DialContext(ctx, "tcp", broker)
	if err != nil {
		panic(err.Error())
	}
	defer conn.Close()


	partitions, err := conn.ReadPartitions()
	if err != nil {
		panic(err.Error())
	}

	m := map[string]struct{}{}

	for _, p := range partitions {
		m[p.Topic] = struct{}{}
	}

    topic := ""
	for k := range m {
		debug(k)
		if strings.Contains(k, "-alerts-none") {
			topic = k
			break
		}
	}
	//
	if kafkatopic == "" {
		kafkatopic = topic
	}	
	topic = kafkatopic	
	if topic == "" {
		printfRed("\rTopic with alerts-none not found                                          ")
		panic(err)
	}
	debug("\nKAFKA Topic: "+topic+"\n")
	return password, topic, broker
}

// Checking result after an action command
// OK
func CheckOK(action, check, result string) {
	// send action, send check, loop waiting few seconds on result 
	show, _ := bash(action)			// send the first action
	printf("\r   Checking ...")
	debug(show)
	for {							// loop forever
		rc, _ := bash(check)		// send the check command to verify the result
		rc = strings.TrimSpace(rc)
		printf("\r"+rc)
		if rc == result {			// check result
			break
		}   
		time.Sleep(3 * time.Second)	// sleep 3 seconds 
	}
	return
}

// Accessing the datalayer API
// OK
func DataLayerAccess()(string, string) {
	// Get access to datalayer interface
	project := "oc project " + projectN
	show, _ := bash(project)
	debug("\r" + string(currentC) +" | "+ show)
	user, _ := bash(`oc get secret aiops-ir-core-ncodl-api-secret -o jsonpath='{.data.username}' | base64 --decode`)
	pass, _ := bash(`oc get secret aiops-ir-core-ncodl-api-secret -o jsonpath='{.data.password}' | base64 --decode`)
	user = strings.TrimSuffix(user, "\n")
	pass = strings.TrimSuffix(pass, "\n")
	//userpass := user+":"+pass
	userpass :=base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
	debug ("\r"+ userpass)
	urlDatalayer, _ := bash(`oc get route datalayer-api  -o jsonpath='{.status.ingress[0].host}'`)
	return userpass, urlDatalayer
}

// Extracting story columns
// OK
func extractStories(s string) string {
	// extract stories from a JSON payload
	s1 := gjson.Get(s, "lastChangedTime").String()
	s2 := gjson.Get(s, "state").String()
	s3 := gjson.Get(s, "priority").String()
	s4 := gjson.Get(s, "owner").String()
	s5 := gjson.Get(s, "team").String()
	s6 := gjson.Get(s, "title").String()
	out := sprintf("%-25s | %-12s | %-3s | %-12s |  %-12s |  %-80s", s1, s2, s3, s4, s5, s6) 
	return out
}

// Extracting alert columns
// OK
func extractAlerts(s string) string {
	// extract alerts from a JSON payload
	if s == "" {
		return ""
	}
	s1 := gjson.Get(s, "lastStateChangeTime").String()
	s2 := gjson.Get(s, "state").String()
	s3 := gjson.Get(s, "severity").String()
	s4 := gjson.Get(s, "resource.name").String()
	// s5 := gjson.Get(s, "type.classification").String()
	s6 := gjson.Get(s, "summary").String()
	out := sprintf("%-25s | %-8s | %-3s | %-25s | %-80s", s1, s2, s3, s4, s6) 
	return out
}

// Implemented the datalayer API (not installed by default)
// OK
func addRouteDatalayer()  {
	write("apiVersion: route.openshift.io/v1")
	write("kind: Route")
	write("metadata:")
	write("  name: datalayer-api")
	write("  namespace: " + projectN)
	write("spec:")
	write("  port:")
	write("    targetPort: secure-port")
	write("  tls:")
	write("    insecureEdgeTerminationPolicy: Redirect")
	write("    termination: reencrypt")
	write("  to:")
	write("    kind: Service")
	write("    name: aiops-ir-core-ncodl-api")
	write("    weight: 100")
	write("  wildcardPolicy: None")

	bash("oc apply -f addroute.yaml")
	err := os.Remove("addroute.yaml")
    if err != nil {
        log.Fatal(err)
	}
}


// Send an alert using KAFKA
// OK
func sendAlert(s string) string {

	// ##########################################################################
	// KAFKA Consumer
	// Input : JSON payload
	// Output : Colored summary alert string  
	// ##########################################################################

	rt :=""
	if strings.Contains(s, "sender") != true  {
		rt = ""
		return rt 
	}   

	mechanism, err := scram.Mechanism(scram.SHA512, "cp4waiops-cartridge-kafka-auth-0", kafkapass)
	if err != nil {
		panic(err)
	}
	cert, err := ioutil.ReadFile("./ca.crt")
    if err != nil {
        log.Fatal(err)
    }

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cert)
	conf := &tls.Config{RootCAs: certPool,}
	dialer := &kafka.Dialer{SASLMechanism: mechanism, TLS: conf,}

	// Configuration Producer (broker, topic)
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{broker},
		Topic:    kafkatopic,
		Dialer:   dialer,
		Balancer: &kafka.LeastBytes{},
	})
	defer writer.Close()
  
	err = writer.WriteMessages(context.Background(),
	  kafka.Message{
		  Value: []byte(s),
	  },
	)

	if err != nil {
		rt = sprintf("\n%s \n\033[[1;31mERROR\033[0m",s)
	} else {
		timestamp := gjson.Get(s, "occurrenceTime").String()
		summary   := gjson.Get(s, "summary").String()
		severity   := gjson.Get(s, "severity").String()
		rt = sprintf("\033[1;32mAlert Sent | %s |  %s | %s\033[0m",timestamp, severity, summary )
	}

	return rt

}

// Simplified http API request like cURL
// OK
func httpSend(verb, rout, auth, jobi, body  string) (stat, data string) {

	// ##########################################################################
	//
	// INPUT:
	// verb: GET, POST, PATCH, DELETE, PUT ...		HTML Verb
	// rout: https://...							url
	// auth: BASIC+userpass or BEARER+ca ...		Authorization
	// jobi: restTopology							JobId
	// body: JSON data								HTML Body
	//
	// OUTPUT:
	// stat: 200, 202, 403 ...						Response Status Code
	// data: JSON data 								Data returned by the request
	// ##########################################################################

	// Check variables
	debug("VERB:\t"+verb)
	debug("ROUT:\t"+rout)
	debug("AUTH:\t"+auth)
	debug("JOBI:\t"+jobi)
	debug("BODY:\t"+body)

	if (verb == "") || (rout == "") || (auth == "") {
		printfRed("ERROR - HTTP verb or URL is empty")
		panic("HTTP verb or URL is empty")	
	}

	// Transport
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// Get the body
	bodyB := []byte(body)

	// Create a http client and request
	client := &http.Client{Transport: tr}
	hreq, err := http.NewRequest(verb, rout, bytes.NewBuffer(bodyB))
	if err != nil {
		printfRed("ERROR - HTTP Client or Request creation")
		panic(err)
	}

	// Specify headers
	hreq.Header.Add("Authorization",auth) 
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("X-Username", "admin")
	hreq.Header.Set("X-Subscription-Id", "cfd95b7e-3bc7-4006-a4a8-a73a79c71255")
	hreq.Header.Set("X-Tenantid", "cfd95b7e-3bc7-4006-a4a8-a73a79c71255")
	hreq.Header.Set("Accept", "application/json")
	if jobi != "" {
		hreq.Header.Set("JobId", jobi)
	}

	// Send the HTTP request
	resp, err := client.Do(hreq)
	if err != nil {
		printfRed("ERROR - HTTP request problem")
		panic(err)
	}
	defer resp.Body.Close()

	// Check Status and Data
	stat 		= resp.Status
	textB,_ 	:= ioutil.ReadAll(resp.Body)
	data	 	= string(textB)

	debug("STAT: "+stat)
	debug("DATA: "+data)
	
	return stat, data
}

