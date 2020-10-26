package main

// A simple runanalyzer doing the below actions.
//
// 1. list last 3 builds abort jobs
// 2. Total duration of a build cycle
// 3. Execute a given CB N1QL query
// 4. Save the jenkins logs to S3. List of jobs can csv or CB server for a build
//
// jagadesh.munta@couchbase.com

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/magiconair/properties"
	"gopkg.in/ini.v1"
)

// N1QLQryResult type
type N1QLQryResult struct {
	Status  string
	Results []N1QLResult
}

// N1QLResult type
type N1QLResult struct {
	Aname    string
	JURL     string
	URLbuild int64
}

// TotalCycleTimeQryResult type
type TotalCycleTimeQryResult struct {
	Status  string
	Results []TotalCycleTime
}

// TotalCycleTime type
type TotalCycleTime struct {
	Build      string
	Numofjobs  int
	Totaltime  int64
	Failcount  int
	Totalcount int
}

// TotalJobsQryResult type
type TotalJobsQryResult struct {
	Status  string
	Results []TotalJobs
}

// TotalJobs type
type TotalJobs struct {
	TotalNumofjobs int
}

// CBBuildQryResult type
type CBBuildQryResult struct {
	Status  string
	Results []CBBuild
}

// CBBuild type
type CBBuild struct {
	Build string
}

// JobStatusQryResult type
type JobStatusQryResult struct {
	Status  string
	Results []JobStatus
}

// JobStatus type
type JobStatus struct {
	Result    string
	Numofjobs int
}

// JenkinsComputerQryResult type
type JenkinsComputerQryResult struct {
	BusyExecutors  int
	Computer       []JenkinsComputer
	TotalExecutors int
}

// JenkinsComputer type
type JenkinsComputer struct {
	AssignedLabels     []AssignedLabelName
	DisplayName        string
	NumExecutors       int
	Offline            bool
	temporarilyOffline bool
}

// AssignedLabelName type
type AssignedLabelName struct {
	Name string
}

// RerunTotalCycleTimeQryResult type
type RerunTotalCycleTimeQryResult struct {
	Status  string
	Results []RerunTestResult
}

// RerunTestResult type
type RerunTestResult struct {
	Build      string
	FailCount  int
	TotalCount int
	ProdType   string
	BuildOS    OSComponentTestResult
}

// OSComponentTestResult type
type OSComponentTestResult map[string]ComponentTestResult

// ComponentTestResult type
type ComponentTestResult map[string]TestSuiteResult

// TestSuiteResult type
type TestSuiteResult map[string][]TestResult

// TestResult type
type TestResult struct {
	Build_ID   int64
	TClaim     string
	TColor     string
	Deleted    bool
	Disabled   bool
	Duration   int64
	FailCount  int
	OlderBuild bool
	Priority   string
	Result     string
	TotalCount int
	URL        string
}

//const url = "http://172.23.109.245:8093/query/service"

var cbbuild string
var src string
var dest string
var overwrite string
var updateURL string
var cbplatform string
var cbbucket string
var s3bucket string
var url string
var updateOrgURL string
var includes string
var limits string
var finallimits string
var totalmachines string
var qryfilter string
var workspace string
var cbrelease string
var s3url string
var defaultSuiteType string
var qaJenkinsURL string
var requiredServerPools string
var requiredStates string

func main() {
	fmt.Println("*** Helper Tool ***")
	action := flag.String("action", "usage", usage())
	srcInput := flag.String("src", "cbserver", usage())
	destInput := flag.String("dest", "local", usage())
	overwriteInput := flag.String("overwrite", "no", usage())
	updateURLInput := flag.String("updateurl", "no", usage())
	cbplatformInput := flag.String("os", "centos", usage())
	cbbucketInput := flag.String("cbbucket", "server", usage())
	s3bucketInput := flag.String("s3bucket", "cb-logs-qe", usage())
	s3urlInput := flag.String("s3url", "http://cb-logs-qe.s3-website-us-west-2.amazonaws.com/", usage())
	urlInput := flag.String("cbqueryurl", "http://172.23.109.245:8093/query/service", usage())
	qaJenkinsURLInput := flag.String("qajenkins", "http://qa.sc.couchbase.com/", usage())
	updateOrgURLInput := flag.String("updateorgurl", "no", usage())
	includesInput := flag.String("includes", "console,config,parameters,testresult", usage())
	limitsInput := flag.String("limits", "100", usage())
	finallimitsInput := flag.String("finallimits", "100", usage())
	totalmachinesInput := flag.String("totalmachines", "false", usage())
	qryfilterInput := flag.String("qryfilter", " ", usage())
	workspaceInput := flag.String("workspace", "testrunner", usage())
	cbreleaseInput := flag.String("cbrelease", "6.5", usage())
	defaultSuiteTypeInput := flag.String("suite", "12hour", usage())
	requiredServerPoolsInput := flag.String("reqserverpools",
		"regression,durability,ipv6,ipv6-raw,ipv6-fqdn,ipv6-mix,jre-less,jre,security,elastic-fts,elastic-xdcr", usage())
	requiredStatesInput := flag.String("reqstates", "available,booked", usage())

	flag.Parse()
	dest = *destInput
	src = *srcInput
	overwrite = *overwriteInput
	updateURL = *updateURLInput
	cbplatform = *cbplatformInput
	cbbucket = *cbbucketInput
	s3bucket = *s3bucketInput
	s3url = *s3urlInput
	url = *urlInput
	updateOrgURL = *updateOrgURLInput
	includes = *includesInput
	limits = *limitsInput
	finallimits = *finallimitsInput
	totalmachines = *totalmachinesInput
	qryfilter = *qryfilterInput
	workspace = *workspaceInput
	cbrelease = *cbreleaseInput
	defaultSuiteType = *defaultSuiteTypeInput
	qaJenkinsURL = *qaJenkinsURLInput
	requiredServerPools = *requiredServerPoolsInput
	requiredStates = *requiredStatesInput

	//fmt.Println("original dest=", dest, "--", *destInput)
	//time.Sleep(10 * time.Second)
	if *action == "lastaborted" {
		lastabortedjobs()
	} else if *action == "savejoblogs" {
		savejoblogs()
	} else if *action == "totaltime" {
		gettotalbuildcycleduration(os.Args[3])
		//fmt.Printf("\n\t\t\t\t\t\t\t\t\t\t\tGrand total time: %d hours\n", gettotalbuildcycleduration(os.Args[3]))
	} else if *action == "reruntotaltime" {
		getreruntotalbuildcycleduration(os.Args[3])
		//fmt.Printf("\n\t\t\t\t\t\t\t\t\t\t\tGrand total time: %d hours\n", gettotalbuildcycleduration(os.Args[3]))
	} else if *action == "runquery" {
		fmt.Println("Query Result: ", runquery(os.Args[len(os.Args)-1]))
	} else if *action == "runupdatequery" {
		runupdatequery(os.Args[len(os.Args)-1])
	} else if *action == "setpoolipstate" {
		setPoolState(os.Args[len(os.Args)-2], os.Args[len(os.Args)-1])
	} else if *action == "getpoolipstate" {
		getPoolState(os.Args[len(os.Args)-1])
	} else if *action == "getrunprogress" {
		//GenSummaryForRunProgress(os.Args[len(os.Args)-2], os.Args[len(os.Args)-1])
		cbbuild = os.Args[len(os.Args)-1]
		if strings.HasPrefix(cbbuild, "http") {
			fmt.Println("Getting the build from description of build url: " + cbbuild)
			cbbuild = GetJenkinsLastBuildFromDesc(cbbuild + "/api/json")
		}
		fmt.Println("cbbuild=" + cbbuild)
		if cbbuild != "" {
			GenSummaryForRunProgress(cbbuild)
		} else {
			fmt.Println("Warning: No CB build, skipping...")
		}
	} else if *action == "usage" {
		fmt.Println(usage())
	} else {
		fmt.Println(usage())
	}
}

func usage() string {
	fileName, _ := os.Executable()
	return "Usage: " + fileName + " -h | --help \nEnter action value. \n" +
		"-action lastaborted 6.5.0-4106 6.5.0-4059 6.5.0-4000  : to get the aborted jobs common across last 3 builds. Options: --cbrelease [6.5]specificbuilds --limits 3 --qryfilter 'where numofjobs>900' \n" +
		"-action savejoblogs 6.5.0-4106  : to download the jenkins logs and save in S3 for a given build. " +
		"Options: --dest [local]|s3|none --src csvfile --os centos --overwrite [no]|yes --updateurl [no]|yes --includes [console,config,parameters,testresult],archive" +
		"--s3bucket cb-logs-qe --s3url http://cb-logs-qe.s3-website-us-west-2.amazonaws.com/ --cbqueryurl [http://172.23.109.245:8093/query/service]\n" +
		"-action totaltime 6.5  : to get the total number of jobs, time duration for a given set of  builds in a release, " +
		"-action reruntotaltime 6.6  : to get the total number of jobs, time duration for a given set of  builds in a release, " +
		"Options: --cbbucket [server] --limits [100] --qryfilter 'where result.numofjobs>900 and (totalcount-failcount)*100/totalcount>90'\n" +
		"-action getrunprogress build : to get the summary report on the kickedoff runs for a build. " +
		" Options: --reqserverpools=[regression,durability,ipv6,ipv6-raw,ipv6-fqdn,ipv6-mix,jre-less,jre,security,elastic-fts,elastic-xdcr] --reqstates=[available,booked] \n" +
		"-action runquery 'select * from server where lower(`os`)=\"centos\" and `build`=\"6.5.0-4106\"' : to run a given query statement \n" +
		"-action runupdatequery --cbqueryurl 'http://172.23.105.177:8093/query/service'  \"update \\`QE-server-pool\\` set state='available' where ipaddr='172.23.120.240'\" : to run a given update query statement\n" +
		"-action setpoolipstate state ips : to set given state for the given ips (separated by comma)\n" +
		"-action getpoolipstate ips : to get state for given ips (separated by comma)\n"

}

func runquery(qry string) string {
	//url := "http://172.23.109.245:8093/query/service"
	fmt.Println("ACTION: runquery")
	fmt.Println("query=" + qry)
	localFileName := "qryresult.json"
	if err := executeN1QLStmt(localFileName, url, qry); err != nil {
		panic(err)
	}

	resultFile, err := os.Open(localFileName)
	if err != nil {
		fmt.Println(err)
	}
	defer resultFile.Close()

	byteValue, _ := ioutil.ReadAll(resultFile)
	return string(byteValue)
}

func runupdatequery(qry string) {
	//url := "http://172.23.109.245:8093/query/service"
	fmt.Println("ACTION: runupdatequery")
	fmt.Println("query=" + qry)
	if err := executeN1QLPostStmt(url, qry); err != nil {
		panic(err)
	}
}

func setPoolState(state string, ips string) {
	url = "http://172.23.105.177:8093/query/service"
	ipa := strings.Split(ips, ",")
	allIps := ""
	for i := 0; i < len(ipa); i++ {
		if i < len(ipa)-1 {
			allIps += "'" + ipa[i] + "',"
		} else {
			allIps += "'" + ipa[i] + "'"
		}

	}
	qry := "update `QE-server-pool` set state='" + state + "' where ipaddr in [" + allIps + "]"
	fmt.Println(qry)
	runupdatequery(qry)
}

func getPoolState(ips string) {
	url = "http://172.23.105.177:8093/query/service"
	ipa := strings.Split(ips, ",")
	allIps := ""
	for i := 0; i < len(ipa); i++ {
		if i < len(ipa)-1 {
			allIps += "'" + ipa[i] + "',"
		} else {
			allIps += "'" + ipa[i] + "'"
		}

	}
	qry := "select ipaddr,state,poolId from `QE-server-pool` where ipaddr in [" + allIps + "]"
	fmt.Println(qry)
	fmt.Println("Query Result: ", runquery(qry))
}

func gettotalbuildcycleduration(buildN string) int {
	//fmt.Println("action: totaltime")

	var build1 string
	//var builds string
	if len(os.Args) < 2 {
		fmt.Println("Enter the build to get the number of jobs and total duration.")
		os.Exit(1)
	} else {
		build1 = os.Args[len(os.Args)-1]
		cbbuild = build1
		//builds = build1
	}
	var totalhours int

	// For all build releases
	/*
		builds = ""
		var cbbuildrel = strings.Split(build1, ",")
		for i := 0; i < len(cbbuildrel); i++ {
			cbbuild = cbbuildrel[i]
			// get build ids
			qry1 := "select distinct `build` from server where lower(os)like \"" + cbplatform + "\" and `build` like \"" + cbbuild + "%\"  order by `build` desc limit " + limits
			localFileName1 := "cbbuilds.json"
			if err1 := executeN1QLStmt(localFileName1, url, qry1); err1 != nil {
				panic(err1)
			}

			resultFile1, err1 := os.Open(localFileName1)
			if err1 != nil {
				fmt.Println(err1)
			}
			defer resultFile1.Close()

			byteValue1, _ := ioutil.ReadAll(resultFile1)

			var result1 CBBuildQryResult

			err1 = json.Unmarshal(byteValue1, &result1)
			if result1.Status == "success" {

				for i := 0; i < len(result1.Results); i++ {
					builds += result1.Results[i].Build
					if i < len(result1.Results)-1 {
						builds += ","
					}
					//fmt.Printf("buildid=%s, builds=%s", result1.Results[i].Build, builds)

				}
			}

		}
	*/
	// total jobs
	//var cbbuilds = strings.Split(builds, ",")
	outFile, _ := os.Create("totaltime_summary.txt")
	outW := bufio.NewWriter(outFile)
	defer outFile.Close()

	fmt.Printf("\nSummary report of regression cycles on the last %s build(s) in %s %s on %s\n", limits, cbbuild, qryfilter, cbplatform)
	fmt.Fprintf(outW, "\nSummary report of regression cycles on the last %s build(s) in %s %s on %s\n", limits, cbbuild, qryfilter, cbplatform)

	if totalmachines == "true" {
		fmt.Println("---------------------------------------------------------------------------------------------------------------------------------------------------------------------")
		fmt.Println("S.No.\tBuild\t\tTestCount\tPassedCount\tFailedCount\tPassrate\tJobcount(A,F,U,S)\tTotaltime\t\t\t\tMachinesCount")
		fmt.Println("---------------------------------------------------------------------------------------------------------------------------------------------------------------------")
		fmt.Fprintln(outW, "---------------------------------------------------------------------------------------------------------------------------------------------------------------------")
		fmt.Fprintln(outW, "S.No.\tBuild\t\tTestCount\tPassedCount\tFailedCount\tPassrate\tJobscount(A,F,U,S)\tTotaltime\t\t\t\tMachinesCount")
		fmt.Fprintln(outW, "---------------------------------------------------------------------------------------------------------------------------------------------------------------------")
	} else {
		fmt.Println("-----------------------------------------------------------------------------------------------------------------------------------------------------")
		fmt.Println("S.No.\tBuild\t\tTestCount\tPassedCount\tFailedCount\tPassrate\tJobcount(A,F,U,S)\tTotaltime")
		fmt.Println("-----------------------------------------------------------------------------------------------------------------------------------------------------")
		fmt.Fprintln(outW, "-----------------------------------------------------------------------------------------------------------------------------------------------------")
		fmt.Fprintln(outW, "S.No.\tBuild\t\tTestCount\tPassedCount\tFailedCount\tPassrate\tJobscount(A,F,U,S)\tTotaltime")
		fmt.Fprintln(outW, "-----------------------------------------------------------------------------------------------------------------------------------------------------")
	}

	sno := 1
	//for i := 0; i < len(cbbuilds); i++ {
	//	cbbuild = cbbuilds[i]

	//url := "http://172.23.109.245:8093/query/service"
	//qry := "select count(*) as numofjobs, sum(duration) as totaltime, sum(failCount) as failcount, sum(totalCount) as totalcount from server b where lower(b.os) like \"" + cbplatform + "\" and b.`build`=\"" + cbbuild + "\" " + qryfilter
	//qry := "select numofjobs, totaltime, failcount, totalcount from (select count(*) as numofjobs, sum(duration) as totaltime, sum(failCount) as failcount, sum(totalCount) as totalcount from server b " +
	//	"where lower(b.os) like \"" + cbplatform + "\" and b.`build`=\"" + cbbuild + "\" ) as result " + qryfilter + " limit " + finallimits
	qry := "select `build`, numofjobs, totaltime, failcount, totalcount from (select b.`build`, count(*) as numofjobs, sum(duration) as totaltime, sum(failCount) as failcount, sum(totalCount) as totalcount from " + cbbucket + " b " +
		"where lower(b.os) like \"" + cbplatform + "\" and b.`build` like \"" + cbbuild + "%\" group by b.`build` order by b.`build` desc) as result " + qryfilter + " limit " + limits
	//qry := "select `build`, numofjobs, totaltime, failcount, totalcount from (select b.`build`, count(*) as numofjobs, sum(duration) as totaltime, sum(failCount) as failcount, sum(totalCount) as totalcount from server b where lower(b.os) like "centos" and b.`build` like "6.5%" group by b.`build` order by b.`build` desc) as result where numofjobs>500 limit 30"
	fmt.Println("\nquery=" + qry)
	localFileName := "duration.json"
	if err := executeN1QLStmt(localFileName, url, qry); err != nil {
		//panic(err)
		log.Println(err)
	}
	resultFile, err := os.Open(localFileName)
	if err != nil {
		fmt.Println(err)
	}
	defer resultFile.Close()

	byteValue, _ := ioutil.ReadAll(resultFile)

	var result TotalCycleTimeQryResult

	err = json.Unmarshal(byteValue, &result)

	//if len(result.Results) < 1 {
	//	continue
	//}
	if result.Status == "success" {
		//fmt.Println(" Total time in millis: ", result.Results[0].Totaltime)

		for i := 0; i < len(result.Results); i++ {
			cbbuild = result.Results[i].Build

			// get jobs status
			abortedJobs, failureJobs, unstableJobs, successJobs := getJobsStatusList(cbbuild)

			hours := math.Floor(float64(result.Results[i].Totaltime) / 1000 / 60 / 60)
			totalhours += int(hours)
			secs := result.Results[i].Totaltime % (1000 * 60 * 60)
			mins := math.Floor(float64(secs) / 60 / 1000)
			//secs = result.Results[i].Totaltime * 1000 % 60
			passCount := result.Results[i].Totalcount - result.Results[i].Failcount

			fmt.Printf("\n%3d.\t%s\t%5d\t\t%5d\t\t%5d\t\t%6.2f%%\t\t%3d(%3d,%3d,%3d,%3d)\t%4d hrs %2d mins (%11d millis)",
				(sno), cbbuild, result.Results[i].Totalcount, passCount, result.Results[i].Failcount,
				(float32(passCount)/float32(result.Results[i].Totalcount))*100, result.Results[i].Numofjobs, abortedJobs, failureJobs, unstableJobs, successJobs, int64(hours), int64(mins), result.Results[i].Totaltime)
			fmt.Fprintf(outW, "\n%3d.\t%s\t%5d\t\t%5d\t\t%5d\t\t%6.2f%%\t\t%3d(%3d,%3d,%3d,%3d)\t%4d hrs %2d mins (%11d millis)",
				(sno), cbbuild, result.Results[i].Totalcount, passCount, result.Results[i].Failcount,
				(float32(passCount)/float32(result.Results[i].Totalcount))*100, result.Results[i].Numofjobs, abortedJobs, failureJobs, unstableJobs, successJobs, int64(hours), int64(mins), result.Results[i].Totaltime)

			//get machines list
			totalMachinesCount := 0
			if totalmachines == "true" {
				totalMachinesCount = getMachinesList(cbbuild)
				if totalMachinesCount != 0 {
					fmt.Printf("\t%4d", totalMachinesCount)
					fmt.Fprintf(outW, "\t%4d", totalMachinesCount)
				}
			}

			sno++
			//fmt.Printf("\n%d. %s, Number of jobs=%d, Total duration=%02d hrs %02d mins (%02d millis)", i, cbbuild, result.Results[0].Numofjobs, int64(hours), int64(mins), result.Results[0].Totaltime)
			//fmt.Printf("Number of jobs=%d, Total duration=%02d hrs : %02d mins :%02d secs", result.Results[0].Numofjobs, int64(hours), int64(mins), int64(secs))
			//ttime = string(hours) + ": " + string(mins) + ": " + string(secs)
			outW.Flush()
		}
	} else {
		fmt.Println("Status: Failed. " + err.Error())
	}
	//}
	if totalmachines == "true" {
		fmt.Println("\n---------------------------------------------------------------------------------------------------------------------------------------------------------------------")
		fmt.Fprintln(outW, "\n---------------------------------------------------------------------------------------------------------------------------------------------------------------------")
	} else {
		fmt.Println("\n-----------------------------------------------------------------------------------------------------------------------------------------------------")
		fmt.Fprintln(outW, "\n-----------------------------------------------------------------------------------------------------------------------------------------------------")
	}
	p := fmt.Println
	t := time.Now()
	p(t.Format(time.RFC3339))
	fmt.Fprintf(outW, "\n%s\n", t.Format(time.RFC3339))
	//fmt.Fprintf(outW, "\n%s\t\t\t\t\t\t\t\t\t\tGrand total time=%6d hours\n", t.Format(time.RFC3339), totalhours)

	outW.Flush()

	return totalhours

}

func getreruntotalbuildcycleduration(buildN string) int {
	//fmt.Println("action: totaltime")

	var build1 string
	//var builds string
	if len(os.Args) < 2 {
		fmt.Println("Enter the build to get reruns total duration.")
		os.Exit(1)
	} else {
		build1 = os.Args[len(os.Args)-1]
		cbbuild = build1
		//builds = build1
	}
	var totalhours int

	// total jobs
	//var cbbuilds = strings.Split(builds, ",")
	outFile, _ := os.Create("totaltime_summary.txt")
	outW := bufio.NewWriter(outFile)
	defer outFile.Close()

	outFileCsv, _ := os.Create("totaltime_summary.csv")
	outWCsv := bufio.NewWriter(outFileCsv)
	defer outFileCsv.Close()

	outClockTimeFile, _ := os.Create("clocktime_summary.txt")
	outClockW := bufio.NewWriter(outClockTimeFile)
	defer outClockTimeFile.Close()

	fmt.Printf("\nSummary report of regression cycles on the last %s build(s) in %s %s\n", limits, cbbuild, qryfilter)
	fmt.Fprintf(outW, "\nSummary report of regression cycles on the last %s build(s) in %s %s\n", limits, cbbuild, qryfilter)

	fmt.Println("----------------------------------------------------------------------------------------------------------------------------" +
		"-------------------------------------------------------------------------------------------------------------------------------------" +
		"--------------------------------------------------------------------------------------------------------------------------------------" +
		"---------------------------------------------------------")
	//fmt.Println("S.No.\tBuild\t\tOS\tTestCount\tFailedCount\tPassedCount\tPassrate\tJobcount(A,F,U,S)\tTotaltime\tTotalComponents\tTotalJobs\tTotalRuns\tTotalReruns\tTotalRerunJobs")
	fmt.Println("S.No.\tBuild\t\tOS\tTC\tFC\tPC\tRate\tAborted,Failed,Unstable,Succ\tTotalTime\t#Comp\t#Jobs\t#Runs\t#Reruns\t#RerunJobs RerunRate\tRerunTime" +
		"\t\tFirstRunStartTime\t\tFirstRunEndTime\t\t\tFirstClockTime\tRunStartTime\t\t\tRunEndTime\t\t\tClockTime\tRerunStartTime\t\t\tRerunEndTime\t\t\tRerunClockTime")
	fmt.Println("----------------------------------------------------------------------------------------------------------------------------" +
		"-------------------------------------------------------------------------------------------------------------------------------------" +
		"--------------------------------------------------------------------------------------------------------------------------------------" +
		"---------------------------------------------------------")
	fmt.Fprintln(outW, "----------------------------------------------------------------------------------------------------------------------------"+
		"-------------------------------------------------------------------------------------------------------------------------------------"+
		"--------------------------------------------------------------------------------------------------------------------------------------"+
		"---------------------------------------------------------")
	fmt.Fprintln(outW, "S.No.\tBuild\t\tOS\tTC\tFC\tPC\tRate\tAborted,Failed,Unstable,Succ\tTotalTime\t#Comp\t#Jobs\t#Runs\t#Reruns\t#RerunJobs RerunRate\tRerunTime"+
		"\t\tFirstRunStartTime\t\tFirstRunEndTime\t\t\tFirstClockTime\tRunStartTime\t\t\tRunEndTime\t\t\tClockTime\tRerunStartTime\t\t\tRerunEndTime\t\t\tRerunClockTime")
	fmt.Fprintln(outW, "----------------------------------------------------------------------------------------------------------------------------"+
		"-------------------------------------------------------------------------------------------------------------------------------------"+
		"--------------------------------------------------------------------------------------------------------------------------------------"+
		"---------------------------------------------------------")

	fmt.Fprintln(outClockW, "\n\tComponent\tTestSuite\t\tJenkinBuildURL\t\tStart\t\tEnd\t\tClockDuration")
	fmt.Fprintln(outClockW, "--------------------------------------------------------------------------------------------------------------------")

	fmt.Fprintln(outWCsv, "S.No.,Build,GrandTC,GrandFC,OS,TC,FC,PC,Rate,Aborted,Failed,Unstable,Succ,TotalTime,#Comp,#Jobs,#Runs,#Reruns,#RerunJobs,RerunRate,RerunTime"+
		",FirstRunStartTime,FirstRunEndTime,FirstClockTime,RunStartTime,RunEndTime,ClockTime,RerunStartTime,RerunEndTime,RerunClockTime")

	sno := 1
	//for i := 0; i < len(cbbuilds); i++ {
	//	cbbuild = cbbuilds[i]

	//url := "http://172.23.109.245:8093/query/service"
	//qry := "select count(*) as numofjobs, sum(duration) as totaltime, sum(failCount) as failcount, sum(totalCount) as totalcount from server b where lower(b.os) like \"" + cbplatform + "\" and b.`build`=\"" + cbbuild + "\" " + qryfilter
	//qry := "select numofjobs, totaltime, failcount, totalcount from (select count(*) as numofjobs, sum(duration) as totaltime, sum(failCount) as failcount, sum(totalCount) as totalcount from server b " +
	//	"where lower(b.os) like \"" + cbplatform + "\" and b.`build`=\"" + cbbuild + "\" ) as result " + qryfilter + " limit " + finallimits
	qry := "select `build`, totalCount, failCount, `type`, os as buildOS from test_eventing b " +
		"where  b.`build` like \"" + cbbuild + "%\" " + qryfilter + " order by `build` desc"
	//qry := "select `build`, numofjobs, totaltime, failcount, totalcount from (select b.`build`, count(*) as numofjobs, sum(duration) as totaltime, sum(failCount) as failcount, sum(totalCount) as totalcount from server b where lower(b.os) like "centos" and b.`build` like "6.5%" group by b.`build` order by b.`build` desc) as result where numofjobs>500 limit 30"
	//fmt.Println("\nquery=" + qry)
	localFileName := "duration.json"
	if err := executeN1QLStmt(localFileName, url, qry); err != nil {
		//panic(err)
		log.Println(err)
	}
	resultFile, err := os.Open(localFileName)
	if err != nil {
		fmt.Println(err)
	}
	defer resultFile.Close()

	byteValue, _ := ioutil.ReadAll(resultFile)

	var result RerunTotalCycleTimeQryResult

	err = json.Unmarshal(byteValue, &result)

	//if len(result.Results) < 1 {
	//	continue
	//}
	if result.Status == "success" {
		//fmt.Println(" Total time in millis: ", result.Results[0].Totaltime)

		for i := 0; i < len(result.Results); i++ {
			cbbuild = result.Results[i].Build

			for key, value := range result.Results[i].BuildOS {
				//fmt.Println("OS:", key, "Value:", value, "# of Jobs/Suites:", len(value))

				totalRuns := 0
				totalReruns := 0
				totalJobs := 0
				reranJobCount := 0
				totalTestCount := 0
				totalFailCount := 0
				totalAborted := 0
				totalFailed := 0
				totalUnstable := 0
				totalSuccess := 0
				var totalRerunOnlyDuration int64
				var totalGrandDuration int64
				var totalFirstDuration int64
				var reranJobsList string
				var startTimeStamp int64
				var endTimeStamp int64
				var rerunStartTimeStamp int64
				var rerunEndTimeStamp int64
				var firstStartTimeStamp int64
				var firstEndTimeStamp int64
				startTimeStamp = 0
				rerunStartTimeStamp = 0
				endTimeStamp = 0
				rerunEndTimeStamp = 0
				firstStartTimeStamp = 0
				firstEndTimeStamp = 0
				jobIndex := 1
				for key1, value1 := range value {
					//fmt.Println("\nComponent:", key1, "Value1:", value1)
					fmt.Fprintf(outClockW, "\n\t%s \t%s \n", cbbuild, key)
					totalJobs += len(value1)
					for key2, value2 := range value1 {
						//fmt.Println("\nJob/Suite:", key2, "Value2:", value2)
						tests := make([]TestResult, 10)
						rerunCount := copy(tests, value2)
						if rerunCount > 1 {
							reranJobCount++
							reranJobsList += fmt.Sprintf("\n\t%d. %s: %s -- %d ", reranJobCount, key1, key2, (rerunCount - 1))
							for i := 0; i < len(tests)-1; i++ {
								totalRerunOnlyDuration += tests[i].Duration
								if tests[i].URL != "" {

									downloadURL := ""
									if strings.Contains(tests[i].URL, strings.TrimSpace(strconv.FormatInt(tests[i].Build_ID, 10))) {
										downloadURL = fmt.Sprintf("%s", tests[i].URL)
									} else {
										downloadURL = fmt.Sprintf("%s%d", tests[i].URL, tests[i].Build_ID)
									}
									//fmt.Println("\nRerun Download..." + downloadURL)
									//fmt.Printf("\nOverall Download...%s", downloadURL)
									jsonValue := DownloadJenkinsURLContent(downloadURL + "/api/json?prretty=true")
									var jenkinsBuild JenkinsBuildInfo
									err := json.Unmarshal(jsonValue, &jenkinsBuild)
									if err == nil {
										if rerunStartTimeStamp == 0 || rerunStartTimeStamp > jenkinsBuild.Timestamp {
											rerunStartTimeStamp = jenkinsBuild.Timestamp
											//fmt.Printf("\nRerun Download...%s", downloadURL)
										}
										if rerunEndTimeStamp < (jenkinsBuild.Timestamp + jenkinsBuild.Duration) {
											rerunEndTimeStamp = jenkinsBuild.Timestamp + jenkinsBuild.Duration
										}

									}
								}
							}
						}
						for i := 0; i < len(tests); i++ {
							totalGrandDuration += tests[i].Duration
							if tests[i].URL != "" &&
								!strings.Contains(tests[i].URL, "cv.jenkins.couchbase.com") &&
								!strings.Contains(tests[i].URL, "server.jenkins.couchbase.com") {
								downloadURL := ""
								if strings.Contains(tests[i].URL, strings.TrimSpace(strconv.FormatInt(tests[i].Build_ID, 10))) {
									downloadURL = fmt.Sprintf("%s", tests[i].URL)
								} else {
									downloadURL = fmt.Sprintf("%s%d", tests[i].URL, tests[i].Build_ID)
								}
								//fmt.Printf("\nOverall Download...%s", downloadURL)
								jsonValue := DownloadJenkinsURLContent(downloadURL + "/api/json?prretty=true")
								var jenkinsBuild JenkinsBuildInfo
								err := json.Unmarshal(jsonValue, &jenkinsBuild)
								if err == nil {
									if startTimeStamp == 0 || startTimeStamp > jenkinsBuild.Timestamp {
										startTimeStamp = jenkinsBuild.Timestamp
										//fmt.Printf("\nOverall Download...%s", downloadURL)
									}
									if endTimeStamp < (jenkinsBuild.Timestamp + jenkinsBuild.Duration) {
										endTimeStamp = jenkinsBuild.Timestamp + jenkinsBuild.Duration
									}
									if i == rerunCount-1 { //very first run
										if firstStartTimeStamp == 0 || firstStartTimeStamp > jenkinsBuild.Timestamp {
											firstStartTimeStamp = jenkinsBuild.Timestamp
										}
										if firstEndTimeStamp < (jenkinsBuild.Timestamp + jenkinsBuild.Duration) {
											firstEndTimeStamp = jenkinsBuild.Timestamp + jenkinsBuild.Duration
										}
									}
									//fmt.Printf("\n%d %s", jenkinsBuild.Timestamp, time.Unix(jenkinsBuild.Timestamp/1000, 0))
								}

								pstart1 := time.Unix(jenkinsBuild.Timestamp/1000, 0)
								pend1 := time.Unix((jenkinsBuild.Timestamp+jenkinsBuild.Duration)/1000, 0)
								pclockTime := pend1.Sub(pstart1)
								fmt.Fprintf(outClockW, "\n%d\t%s \t%s \t%s \t%s \t%s \t%s", jobIndex, key1, key2, downloadURL, pstart1, pend1, pclockTime)
								jobIndex++
							}

						}
						totalFirstDuration += tests[rerunCount-1].Duration
						totalFailCount += tests[0].FailCount
						totalTestCount += tests[0].TotalCount
						switch tests[0].Result {
						case "ABORTED":
							totalAborted++
							break
						case "FAILURE":
							totalFailed++
							break
						case "UNSTABLE":
							totalUnstable++
							break
						case "SUCCESS":
							totalSuccess++
							break
						}
						//fmt.Println("Reruns: ", rerunCount)
						totalRuns += rerunCount
						totalReruns += (rerunCount - 1)
					}
				}
				//fmt.Printf("\nOS:%s \t%d \t%d \t%d \t%d \t%d \t%d \t%d \tComponents=%d \tJobs=%d \tTotal Runs=%d \tReruns=%d \tRerun jobs=%d",
				//	key, totalTestCount, totalFailCount, totalAborted, totalFailed, totalUnstable, totalSuccess, totalDuration, len(value), totalJobs,
				//	totalRuns, totalReruns, reranJobCount)
				var totalPassCount = totalTestCount - totalFailCount
				var totalPassRate = (totalPassCount * 100) / totalTestCount
				var totalComps = len(value)
				hours := math.Floor(float64(totalGrandDuration) / 1000 / 60 / 60)
				secs := totalGrandDuration % (1000 * 60 * 60)
				mins := math.Floor(float64(secs) / 60 / 1000)

				rerunHours := math.Floor(float64(totalRerunOnlyDuration) / 1000 / 60 / 60)
				rerunSecs := totalRerunOnlyDuration % (1000 * 60 * 60)
				rerunMins := math.Floor(float64(rerunSecs) / 60 / 1000)

				var rerunsRate = (totalReruns * 100) / totalRuns

				start1 := time.Unix(startTimeStamp/1000, 0)
				end1 := time.Unix(endTimeStamp/1000, 0)
				clockTime := end1.Sub(start1)

				start2 := time.Unix(rerunStartTimeStamp/1000, 0)
				end2 := time.Unix(rerunEndTimeStamp/1000, 0)
				clockTime2 := end2.Sub(start2)

				start0 := time.Unix(firstStartTimeStamp/1000, 0)
				end0 := time.Unix(firstEndTimeStamp/1000, 0)
				clockTime0 := end0.Sub(start0)

				if totalJobs > 100 {
					fmt.Printf("\n%d\t%s \t\t%d \t%d", (sno), cbbuild, result.Results[i].TotalCount, result.Results[i].FailCount)
					fmt.Fprintf(outW, "\n%d\t%s \t\t%d \t%d", (sno), cbbuild, result.Results[i].TotalCount, result.Results[i].FailCount)
					if rerunHours > 0 || rerunMins > 0 {
						fmt.Printf("\n\t\t\t%s \t%5d \t%5d \t%5d \t%3d%% \t%4d \t%4d \t%4d \t%4d \t%4dhrs:%2dmins \t%3d \t%3d \t%3d \t%3d \t%3d \t%3d%% \t\t%4dhrs:%2dmins  \t%s \t%s \t%s, \t%s \t%s \t%s \t%s \t%s \t%s",
							key, totalTestCount, totalFailCount, totalPassCount, totalPassRate, totalAborted, totalFailed, totalUnstable, totalSuccess, int64(hours), int64(mins), totalComps, totalJobs,
							totalRuns, totalReruns, reranJobCount, rerunsRate, int64(rerunHours), int64(rerunMins), start0, end0, clockTime0, start1, end1, clockTime, start2, end2, clockTime2)
						fmt.Fprintf(outW, "\n\t\t\t%s \t%5d \t%5d \t%5d \t%3d%% \t%4d \t%4d \t%4d \t%4d \t%4dhrs:%2dmins \t%3d \t%3d \t%3d \t%3d \t%3d \t%3d%% \t\t%4dhrs:%2dmins  \t%s \t%s \t%s \t%s \t%s \t%s \t%s \t%s \t%s",
							key, totalTestCount, totalFailCount, totalPassCount, totalPassRate, totalAborted, totalFailed, totalUnstable, totalSuccess, int64(hours), int64(mins), totalComps, totalJobs,
							totalRuns, totalReruns, reranJobCount, rerunsRate, int64(rerunHours), int64(rerunMins), start0, end0, clockTime0, start1, end1, clockTime, start2, end2, clockTime2)
						fmt.Fprintf(outW, "\n%s", reranJobsList)
						fmt.Fprintf(outWCsv, "%d,%s,%d,%d,%s,%d,%d,%d,%d%%,%d,%d,%d,%d,%dhrs:%dmins,%d,%d,%d,%d,%d,%d%%,%dhrs:%dmins,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
							(sno), cbbuild, result.Results[i].TotalCount, result.Results[i].FailCount, key, totalTestCount, totalFailCount, totalPassCount, totalPassRate, totalAborted, totalFailed, totalUnstable, totalSuccess, int64(hours), int64(mins), totalComps, totalJobs,
							totalRuns, totalReruns, reranJobCount, rerunsRate, int64(rerunHours), int64(rerunMins), start0, end0, clockTime0, start1, end1, clockTime, start2, end2, clockTime2)
						//fmt.Fprintf(outWCsv, "\n%s", reranJobsList)
					} else {
						fmt.Printf("\n\t\t\t%s \t%5d \t%5d \t%5d \t%3d%% \t%4d \t%4d \t%4d \t%4d \t%4dhrs:%2dmins \t%3d \t%3d \t%3d \t%3d \t%3d \tstart=%s \tend=%s \t%s \t%s \t%s \t%s \t%s \t%s \t%s",
							key, totalTestCount, totalFailCount, totalPassCount, totalPassRate, totalAborted, totalFailed, totalUnstable, totalSuccess, int64(hours), int64(mins), totalComps, totalJobs,
							totalRuns, totalReruns, reranJobCount, start0, end0, clockTime0, start1, end1, clockTime, start2, end2, clockTime2)
						fmt.Fprintf(outW, "\n\t\t\t%s \t%5d \t%5d \t%5d \t%3d%% \t%4d \t%4d \t%4d \t%4d \t%4dhrs:%2dmins \t%3d \t%3d \t%3d \t%3d \t%3d \t%s \t%s \t%s \t%s \t%s \t%s \t%s \t%s \t%s",
							key, totalTestCount, totalFailCount, totalPassCount, totalPassRate, totalAborted, totalFailed, totalUnstable, totalSuccess, int64(hours), int64(mins), totalComps, totalJobs,
							totalRuns, totalReruns, reranJobCount, start0, end0, clockTime0, start1, end1, clockTime, start2, end2, clockTime2)
						fmt.Fprintf(outWCsv, "%d,%s,%d,%d,%s,%d,%d,%d,%d%%,%d,%d,%d,%d,%dhrs:%dmins,%d,%d,%d,%d,%d,0%,0hrs:0mins,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
							(sno), cbbuild, result.Results[i].TotalCount, result.Results[i].FailCount, key, totalTestCount, totalFailCount, totalPassCount, totalPassRate, totalAborted, totalFailed, totalUnstable, totalSuccess, int64(hours), int64(mins), totalComps, totalJobs,
							totalRuns, totalReruns, reranJobCount, start0, end0, clockTime0, start1, end1, clockTime, start2, end2, clockTime2)
					}
					sno++
				}

			}

			outW.Flush()
			outClockW.Flush()
			outWCsv.Flush()
		}
	} else {
		fmt.Println("Status: Failed. " + err.Error())
	}
	//}
	fmt.Println("\n----------------------------------------------------------------------------------------------------------------------------" +
		"-------------------------------------------------------------------------------------------------------------------------------------" +
		"--------------------------------------------------------------------------------------------------------------------------------------" +
		"---------------------------------------------------------")
	fmt.Fprintln(outW, "\n----------------------------------------------------------------------------------------------------------------------------"+
		"-------------------------------------------------------------------------------------------------------------------------------------"+
		"--------------------------------------------------------------------------------------------------------------------------------------"+
		"---------------------------------------------------------")

	p := fmt.Println
	t := time.Now()
	p(t.Format(time.RFC3339))
	fmt.Fprintf(outW, "\n%s\n", t.Format(time.RFC3339))
	//fmt.Fprintf(outW, "\n%s\t\t\t\t\t\t\t\t\t\tGrand total time=%6d hours\n", t.Format(time.RFC3339), totalhours)

	outW.Flush()
	outFile.Close()
	outClockTimeFile.Close()
	outFileCsv.Close()
	return totalhours

}

func savejoblogs() {
	fmt.Println("action: savejoblogs")
	var build1 string
	if len(os.Args) < 2 {
		fmt.Println("Enter the build to save the jenkins job logs.")
		os.Exit(1)
	} else {
		build1 = os.Args[len(os.Args)-1]
		cbbuild = build1
	}
	var jobCsvFile string
	if src == "cbserver" {
		//url := "http://172.23.109.245:8093/query/service"
		qry := "select b.name as aname,b.url as jurl,b.build_id urlbuild from " + cbbucket + " b where lower(b.os) like \"" + cbplatform + "\" and b.`build`=\"" + build1 + "\""
		fmt.Println("query=" + qry)
		localFileName := "result.json"
		if err := executeN1QLStmt(localFileName, url, qry); err != nil {
			panic(err)
		}

		resultFile, err := os.Open(localFileName)
		if err != nil {
			fmt.Println(err)
		}
		defer resultFile.Close()

		byteValue, _ := ioutil.ReadAll(resultFile)

		var result N1QLQryResult

		err = json.Unmarshal(byteValue, &result)
		//fmt.Println("Status=" + result.Status)
		//fmt.Println(err)
		jobCsvFile = cbbuild + "_all_jobs.csv"
		f, err := os.Create(jobCsvFile)
		defer f.Close()

		w := bufio.NewWriter(f)

		if result.Status == "success" {
			fmt.Println("Count: ", len(result.Results))
			for i := 0; i < len(result.Results); i++ {
				//fmt.Println((i + 1), result.Results[i].Aname, result.Results[i].JURL, result.Results[i].URLbuild)
				fmt.Print(strings.TrimSpace(result.Results[i].Aname), ",", strings.TrimSpace(result.Results[i].JURL), ",",
					result.Results[i].URLbuild, "\n")
				_, err = fmt.Fprintf(w, "%s,%s,%d\n", strings.TrimSpace(result.Results[i].Aname), strings.TrimSpace(result.Results[i].JURL),
					result.Results[i].URLbuild)
			}
			w.Flush()
			fmt.Println("Count: ", len(result.Results))

		} else {
			fmt.Println("Status: Failed")
		}
	} else {
		jobCsvFile = src
	}

	// Download the files
	if !strings.Contains(strings.ToLower(dest), "none") {
		DownloadJenkinsFiles(jobCsvFile)
	}

}

// getJobsList of a given build
func getJobsList(build1 string) string {

	var jobCsvFile string
	if src == "cbserver" {
		//url := "http://172.23.109.245:8093/query/service"
		qry := "select b.name as aname,b.url as jurl,b.build_id urlbuild from " + cbbucket + " b where lower(b.os) like \"" + cbplatform + "\" and b.`build`=\"" + build1 + "\""
		//fmt.Println("query=" + qry)
		localFileName := "jobslistresult.json"
		if err := executeN1QLStmt(localFileName, url, qry); err != nil {
			panic(err)
		}

		resultFile, err := os.Open(localFileName)
		if err != nil {
			fmt.Println(err)
		}
		defer resultFile.Close()

		byteValue, _ := ioutil.ReadAll(resultFile)

		var result N1QLQryResult

		err = json.Unmarshal(byteValue, &result)
		//fmt.Println("Status=" + result.Status)
		//fmt.Println(err)
		jobCsvFile = build1 + "_all_jobs.csv"
		f, err := os.Create(jobCsvFile)
		defer f.Close()

		w := bufio.NewWriter(f)

		if result.Status == "success" {
			fmt.Printf(" Jobs Count: %d\n", len(result.Results))
			for i := 0; i < len(result.Results); i++ {
				//fmt.Println((i + 1), result.Results[i].Aname, result.Results[i].JURL, result.Results[i].URLbuild)
				//fmt.Print(strings.TrimSpace(result.Results[i].Aname), ",", strings.TrimSpace(strings.ReplaceAll(result.Results[i].JURL, "=", "")), ",",
				//	result.Results[i].URLbuild, "\n")
				_, err = fmt.Fprintf(w, "%s,%s,%d\n", strings.TrimSpace(result.Results[i].Aname), strings.TrimSpace(strings.ReplaceAll(result.Results[i].JURL, ",", "")),
					result.Results[i].URLbuild)
				if err != nil {
					log.Println(err)
				}
			}
			w.Flush()
			f.Close()
			//fmt.Println("Count: ", len(result.Results))
			resultFile.Close()
		} else {
			fmt.Println("Status: Failed")
		}
	} else {
		jobCsvFile = src
	}

	return jobCsvFile
}

// getMachinesList...
func getMachinesList(build1 string) int {
	fmt.Printf("\n-->Build: %s ", build1)
	var jobCsvFile = getJobsList(build1)
	// Download the files
	includes = "console,parameters"
	dest = "local"
	return DownloadJenkinsJobInfo(jobCsvFile)
	// Parse the properties file

}

// getJobsStatusList of a given build
func getJobsStatusList(build1 string) (int, int, int, int) {

	abortedJobs := 0
	failureJobs := 0
	unstableJobs := 0
	successJobs := 0
	if src == "cbserver" {
		//url := "http://172.23.109.245:8093/query/service"
		//qry := "select b.name as aname,b.url as jurl,b.build_id urlbuild from server b where lower(b.os) like \"" + cbplatform + "\" and b.`build`=\"" + build1 + "\""
		qry := "select result, count(*) as numofjobs from " + cbbucket + " where lower(os) like \"" + cbplatform + "\"  and `build`=\"" + build1 + "\" group by result"
		//fmt.Println("query=" + qry)
		localFileName := "jobstatusresult.json"
		if err := executeN1QLStmt(localFileName, url, qry); err != nil {
			panic(err)
		}

		resultFile, err := os.Open(localFileName)
		if err != nil {
			fmt.Println(err)
		}
		defer resultFile.Close()

		byteValue, _ := ioutil.ReadAll(resultFile)

		var result JobStatusQryResult

		err = json.Unmarshal(byteValue, &result)

		if result.Status == "success" {
			for i := 0; i < len(result.Results); i++ {
				switch result.Results[i].Result {
				case "ABORTED":
					abortedJobs = result.Results[i].Numofjobs
					break
				case "FAILURE":
					failureJobs = result.Results[i].Numofjobs
					break
				case "UNSTABLE":
					unstableJobs = result.Results[i].Numofjobs
					break
				case "SUCCESS":
					successJobs = result.Results[i].Numofjobs
					break
				}
			}
			resultFile.Close()
		} else {
			fmt.Println("Status: Failed")
		}
	}

	return abortedJobs, failureJobs, unstableJobs, successJobs
}

// DownloadJenkinsJobInfo ...
func DownloadJenkinsJobInfo(csvFile string) int {

	props := properties.MustLoadFile("${HOME}/.jenkins_env.properties", properties.UTF8)
	//jenkinsServer := props.MustGetString("QA_JENKINS_SERVER")
	//jenkinsUser := props.MustGetString("QA_JENKINS_USER")
	//jenkinsUserPwd := props.MustGetString("QA_JENKINS_TOKEN")

	lines, err := ReadCsv(csvFile)
	if err != nil {
		log.Println(err)
		return -1
	}
	index := 0
	numServers := 0
	totalMachines := 0
	machinesOut, machinesErr := os.Create(cbbuild + "_" + cbplatform + "_jobsmachineslist.txt")
	if machinesErr != nil {
		log.Println("Machines list file creation error ", err)
	}
	defer machinesOut.Close()
	listOut := bufio.NewWriter(machinesOut)
	for _, line := range lines {
		data := CSVJob{
			TestName: line[0],
			JobURL:   line[1],
			BuildID:  line[2],
		}
		index++
		//fmt.Println("\n" + strconv.Itoa(index) + "/" + strconv.Itoa(len(lines)) + ". " + data.TestName + " " + data.JobURL + " " + data.BuildID)
		fmt.Printf(".")

		// Start downloading
		req, _ := http.NewRequest("GET", data.JobURL, nil)
		JobName := path.Base(req.URL.Path)
		if JobName == data.BuildID {
			JobName = path.Base(req.URL.Path + "/..")
			data.JobURL = data.JobURL + "../"
		}

		JobDir := cbbuild + "/" + "jenkins_logs" + "/" + JobName + "/" + data.BuildID
		err := os.MkdirAll(JobDir, 0755)
		if err != nil {
			fmt.Println(err)
		}

		ConfigFile := JobDir + "/" + "config.xml"
		JobFile := JobDir + "/" + "jobinfo.json"
		ResultFile := JobDir + "/" + "testresult.json"
		ResultFileXML := JobDir + "/" + "testresult.xml"
		LogFile := JobDir + "/" + "consoleText.txt"
		ArchiveZipFile := JobDir + "/" + "archive.zip"

		URLParts := strings.Split(data.JobURL, "/")
		jenkinsServer := strings.ToUpper(strings.Split(URLParts[2], ".")[0])

		if strings.Contains(strings.ToLower(jenkinsServer), s3bucket) { // from S3
			log.Println("Download from S3...")
			includedFiles := strings.Split(includes, ",")
			for i := 0; i < len(includedFiles); i++ {
				switch strings.ToLower(strings.TrimSpace(includedFiles[i])) {
				case "console":
					//log.Println("...downloading console file")
					DownloadFile(LogFile, data.JobURL+data.BuildID+"/consoleText.txt")
					break
				case "config":
					//log.Println("...downloading config file")
					DownloadFile(ConfigFile, data.JobURL+"/config.xml")
					break
				case "parameters":
					//log.Println("...downloading parameters file")
					DownloadFile(JobFile, data.JobURL+data.BuildID+"/jobinfo.json")
					break
				case "testresult":
					//log.Println("...downloading testresult file")
					DownloadFile(ResultFile, data.JobURL+data.BuildID+"/testresult.json")
					DownloadFile(ResultFileXML, data.JobURL+data.BuildID+"/testresult.xml")
					break
				case "archive":
					//log.Println("...downloading archive file")
					DownloadFile(ArchiveZipFile, data.JobURL+data.BuildID+"/archive.zip")
					break
				}
			}

		} else { // from Jenkins
			//fmt.Println("Jenkins Server: ", jenkinsServer)
			jenkinsUser := props.MustGetString(jenkinsServer + "_JENKINS_USER")
			jenkinsUserPwd := props.MustGetString(jenkinsServer + "_JENKINS_TOKEN")

			includedFiles := strings.Split(includes, ",")
			for i := 0; i < len(includedFiles); i++ {
				switch strings.ToLower(strings.TrimSpace(includedFiles[i])) {
				case "console":
					//log.Println("...downloading console file")
					if !fileExists(LogFile) {
						DownloadFileWithBasicAuth(LogFile, data.JobURL+data.BuildID+"/consoleText", jenkinsUser, jenkinsUserPwd)
					} /*else {
						log.Println("File already existed locally...")
					}*/
					substring := ""
					if fileExists(LogFile) {
						//check if log is available
						//log.Println("SearchingFile2 for No such file string...")
						substring = "No such file:"
						out, _ := SearchFile2(LogFile, substring)
						//log.Println("out=", out, "error=", err)
						if out != "" {
							//log.Println("Jenkins log is not available. Let us check at S3...")
							s3consolelogurl := strings.ReplaceAll(s3url, "cb-logs-qe", s3bucket) + cbbuild + "/jenkins_logs/" + JobName + "/" + data.BuildID + "/consoleText.txt"
							//log.Println("Downloading from " + s3consolelogurl)
							DownloadFile(LogFile, s3consolelogurl)
							if !fileExists(LogFile) {
								//log.Println("No log file at S3 also! skipping." + s3consolelogurl)
								continue
							} else {
								//log.Println("SearchingFile for 404...")
								out, err = SearchFile2(LogFile, "404 Not Found")
								if out != "" {
									log.Println("No log file at S3 also! skipping." + s3consolelogurl)
									break
								}
							}

						} /*else {
							log.Println("Using the local file...")
						}*/
						//log.Println("Searching for INI file...")
						substring = "testrunner -i"
						out, err = SearchFile2(LogFile, substring)
						//log.Println(out, err)

						if out == "" {
							substring = "testrunner.py -i"
							out, err = SearchFile2(LogFile, substring)
							//log.Println(out, err)
						}
						if out != "" {
							lineindex := strings.Index(out, substring)
							substring1 := out[lineindex+len(substring):]
							//log.Println(substring1)
							words := strings.Split(substring1, " ")
							numServers = 0
							if len(words) > 0 {

								iniFile := words[1]
								//log.Println("iniFile=", iniFile)
								if strings.Contains(iniFile, "/tmp") {
									out, _ := SearchFileNextLines2(LogFile, "[servers]")
									//log.Println(out, err)
									numServers = len(strings.Split(out, "\n")) - 1
								} else {
									if _, err := os.Stat(workspace); os.IsNotExist(err) {
										log.Println("The testrunner workspace directory doesn't exist. Performing git clone http://github.com/couchbase/testrunner")
										out, err := exec.Command("git", "clone", "http://github.com/couchbase/testrunner").Output()
										if err != nil {
											log.Fatal(err)
										} else {
											log.Println(out)
										}

									}

									cfg, err := ini.Load(workspace + "/" + iniFile)
									if err != nil {
										fmt.Printf("Fail to read file: %v", err)
										continue
									}

									// Classic read of values, default section can be represented as empty string
									numServers = len(cfg.Section("servers").Keys())
								}
								//fmt.Println("servers:", numServers)
								totalMachines += numServers
								//fmt.Println("Total machines:", totalMachines)
								fmt.Fprintln(listOut, "\n"+strconv.Itoa(index)+"/"+strconv.Itoa(len(lines))+". "+data.TestName+" "+data.JobURL+" "+data.BuildID+"\t"+strconv.Itoa(numServers)+"\t-->"+strconv.Itoa(totalMachines))
								listOut.Flush()
							}
						}

					}
					break
				case "config":
					//log.Println("...downloading config file")
					DownloadFileWithBasicAuth(ConfigFile, data.JobURL+"/config.xml", jenkinsUser, jenkinsUserPwd)
					break
				case "parameters":
					//log.Println("...downloading parameters file")
					DownloadFileWithBasicAuth(JobFile, data.JobURL+data.BuildID+"/api/json?pretty=true", jenkinsUser, jenkinsUserPwd)
					break
				case "testresult":
					//log.Println("...downloading testresult file")
					DownloadFileWithBasicAuth(ResultFile, data.JobURL+data.BuildID+"/testReport/api/json?pretty=true", jenkinsUser, jenkinsUserPwd)
					DownloadFileWithBasicAuth(ResultFileXML, data.JobURL+data.BuildID+"/testReport/api/xml?pretty=true", jenkinsUser, jenkinsUserPwd)
					break
				case "archive":
					//log.Println("...downloading archive file")
					DownloadFileWithBasicAuth(ArchiveZipFile, data.JobURL+data.BuildID+"/artifact/*zip*/archive.zip", jenkinsUser, jenkinsUserPwd)
					break
				}
			}
		}
	}
	machinesOut.Close()

	return totalMachines
}

// executeCommand ...
func executeCommand(command string, input string) string {
	cmdFileWithArgs := strings.Split(command, " ")
	cmdFile := cmdFileWithArgs[0]
	cmdArgs := cmdFileWithArgs[1:]
	cmd := exec.Command(cmdFile, cmdArgs...)
	cmd.Stdin = strings.NewReader(input)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		//log.Fatal(err)
		if out.String() != "" {
			log.Println(err)
		}
	}
	return out.String()
}

func lastabortedjobs() {
	fmt.Println("action: lastaborted")

	var build1 string
	var build2 string
	var build3 string
	var qry string
	var builds = ""
	if cbrelease == "specificbuilds" {
		build1 = os.Args[len(os.Args)-3]
		build2 = os.Args[len(os.Args)-2]
		build3 = os.Args[len(os.Args)-1]
		cbbuild = build1
		qry = "select b.name as aname,b.url as jurl,b.build_id urlbuild from " + cbbucket + " b where lower(b.os) like \"" + cbplatform + "\" and b.result=\"ABORTED\" and b.`build`=\"" +
			build1 + "\" and b.name in (select raw a.name from " + cbbucket + " a where lower(a.os) like \"" + cbplatform + "\" and a.result=\"ABORTED\" and a.`build`=\"" +
			build2 + "\" intersect select raw name from " + cbbucket + " where lower(os) like \"" + cbplatform + "\" and result=\"ABORTED\" and `build`=\"" +
			build3 + "\" intersect select raw name from " + cbbucket + " where lower(os) like \"" + cbplatform + "\" and result=\"ABORTED\" and `build`=\"" + build1 + "\")"
	} else {
		// Get latest builds
		log.Println("Finding latest builds ")
		cbbuilds := getLatestBuilds(cbrelease)
		if len(cbbuilds) < 1 {
			fmt.Println("No builds found!")
			return
		}
		var qryString = ""

		fmt.Printf("Builds: ")
		for i := 0; i < len(cbbuilds); i++ {
			fmt.Printf(cbbuilds[i].Build + " ")
			builds += cbbuilds[i].Build + " "
			qryString += "select raw a.name from " + cbbucket + " a where lower(a.os) like \"" + cbplatform + "\" and a.result=\"ABORTED\" and a.`build`=\"" + cbbuilds[i].Build + "\""
			if i < len(cbbuilds)-1 {
				qryString += " intersect "
			}
		}
		qry = "select b.name as aname,b.url as jurl,b.build_id urlbuild from " + cbbucket + " b where lower(b.os) like \"" + cbplatform + "\" and b.result=\"ABORTED\" and b.`build`=\"" +
			cbbuilds[0].Build + "\" and b.name in (" + qryString + " )"
		fmt.Println("\nquery=" + qry)
	}

	localFileName := "abortedresult.json"
	if err := executeN1QLStmt(localFileName, url, qry); err != nil {
		panic(err)
	}

	resultFile, err := os.Open(localFileName)
	if err != nil {
		fmt.Println(err)
	}
	defer resultFile.Close()

	byteValue, _ := ioutil.ReadAll(resultFile)

	var result N1QLQryResult

	err = json.Unmarshal(byteValue, &result)
	//fmt.Println("Status=" + result.Status)
	//fmt.Println(err)
	f, err := os.Create("aborted_jobs.csv")
	defer f.Close()

	w := bufio.NewWriter(f)
	if result.Status == "success" {
		fmt.Printf("Aborts: %d in %s\n", len(result.Results), builds)
		for i := 0; i < len(result.Results); i++ {
			//fmt.Println((i + 1), result.Results[i].Aname, result.Results[i].JURL, result.Results[i].URLbuild)
			fmt.Printf("\n%s\t%s\t%d", result.Results[i].Aname, result.Results[i].JURL, result.Results[i].URLbuild)
			_, err = fmt.Fprintf(w, "%s,%s,%d\n", strings.TrimSpace(result.Results[i].Aname), strings.TrimSpace(result.Results[i].JURL),
				result.Results[i].URLbuild)
		}
		fmt.Println("")
		w.Flush()

	} else {
		fmt.Println("Status: Failed")
	}
}

// DownloadFile2 will download the given url to the given file.
func executeN1QLStmt(localFilePath string, remoteURL string, statement string) error {

	req, err := http.NewRequest("GET", remoteURL, nil)
	if err != nil {
		return err
	}
	//req.SetBasicAuth("Administrator", "password")
	urlq := req.URL.Query()
	urlq.Add("statement", statement)
	req.URL.RawQuery = urlq.Encode()
	u := req.URL.String()
	//fmt.Println(req.URL.String())
	//fmt.Println(req.BasicAuth())
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if localFilePath != "" {
		out, err := os.Create(localFilePath)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, resp.Body)
		resp.Body.Close()
		return err
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		log.Println(string(body))
		resp.Body.Close()
		return err
	}

}

func getLatestBuilds(cbrelease string) []TotalCycleTime {
	// get last limits number of builds
	qry := "select `build`, numofjobs, totaltime, failcount, totalcount from (select b.`build`, count(*) as numofjobs, sum(duration) as totaltime, sum(failCount) as failcount, sum(totalCount) as totalcount from " + cbbucket + " b " +
		"where lower(b.os) like \"" + cbplatform + "\" and b.`build` like \"" + cbrelease + "%\" group by b.`build` order by b.`build` desc) as result " + qryfilter + " limit " + limits
	//fmt.Println("\nquery=" + qry)
	localFileName := cbrelease + "_lastbuilds.json"
	if err := executeN1QLStmt(localFileName, url, qry); err != nil {
		//panic(err)
		log.Println(err)
	}
	resultFile, err := os.Open(localFileName)
	if err != nil {
		fmt.Println(err)
	}
	defer resultFile.Close()

	byteValue, _ := ioutil.ReadAll(resultFile)

	var result TotalCycleTimeQryResult

	err = json.Unmarshal(byteValue, &result)

	return result.Results
}

// DownloadFile2 will download the given url to the given file.
func executeN1QLPostStmt(remoteURL string, statement string) error {

	stmtStr := "{\"statement\": \"" + statement + "\"}"
	fmt.Println(stmtStr)
	var jsonStr = []byte(stmtStr)
	req, err := http.NewRequest("POST", remoteURL, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	//fmt.Println(req.URL.String())
	//fmt.Println(jsonStr)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	log.Println("Response= " + string(body))
	return err
}

// DownloadFile will download the given url to the given file.
func DownloadFile(localFilePath string, remoteURL string) error {
	resp, err := http.Get(remoteURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := os.Create(localFilePath)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, resp.Body)
	out.Close()
	return err
}

// DownloadFileWithBasicAuth will download the given url to the given file.
func DownloadFileWithBasicAuth(localFilePath string, remoteURL string, userName string, pwd string) error {
	//fmt.Println("Downloading ...", localFilePath, "--", remoteURL, "---", userName, "---", pwd)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", remoteURL, nil)
	//localFilePath := path.Base(req.URL.Path)
	req.SetBasicAuth(userName, pwd)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		//log.Println("Warning: ", remoteURL, " not found!")
		return err
	}
	defer resp.Body.Close()
	out, err := os.Create(localFilePath)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, resp.Body)
	out.Close()
	return err
}

// CSVJob ...
type CSVJob struct {
	TestName string
	JobURL   string
	BuildID  string
}

//DownloadJenkinsFiles ...
func DownloadJenkinsFiles(csvFile string) {
	props := properties.MustLoadFile("${HOME}/.jenkins_env.properties", properties.UTF8)
	//jenkinsServer := props.MustGetString("QA_JENKINS_SERVER")
	//jenkinsUser := props.MustGetString("QA_JENKINS_USER")
	//jenkinsUserPwd := props.MustGetString("QA_JENKINS_TOKEN")

	lines, err := ReadCsv(csvFile)
	if err != nil {
		log.Println(err)
		return
	}
	index := 0
	for _, line := range lines {
		data := CSVJob{
			TestName: line[0],
			JobURL:   line[1],
			BuildID:  line[2],
		}
		index++
		fmt.Println("\n" + strconv.Itoa(index) + "/" + strconv.Itoa(len(lines)) + ". " + data.TestName + " " + data.JobURL + " " + data.BuildID)

		// Start downloading
		req, _ := http.NewRequest("GET", data.JobURL, nil)
		JobName := path.Base(req.URL.Path)
		if JobName == data.BuildID {
			JobName = path.Base(req.URL.Path + "/..")
			data.JobURL = data.JobURL + ".."
		}

		// Update original URL in CB server if required to restore
		if strings.Contains(strings.ToLower(updateOrgURL), "yes") {
			qry := "update `server` set url='" + data.JobURL + "/" + JobName + "/' where `build`='" +
				cbbuild + "' and url like '%/" + JobName + "/' and  build_id=" + data.BuildID
			fmt.Println("CB update is in progress with qry= " + qry)
			if err := executeN1QLPostStmt(url, qry); err != nil {
				panic(err)
			}
			continue
		}

		JobDir := cbbuild + "/" + "jenkins_logs" + "/" + JobName + "/" + data.BuildID
		err := os.MkdirAll(JobDir, 0755)
		if err != nil {
			fmt.Println(err)
		}

		ConfigFile := JobDir + "/" + "config.xml"
		JobFile := JobDir + "/" + "jobinfo.json"
		ResultFile := JobDir + "/" + "testresult.json"
		ResultFileXML := JobDir + "/" + "testresult.xml"
		LogFile := JobDir + "/" + "consoleText.txt"
		ArchiveZipFile := JobDir + "/" + "archive.zip"

		URLParts := strings.Split(data.JobURL, "/")
		jenkinsServer := strings.ToUpper(strings.Split(URLParts[2], ".")[0])

		if strings.Contains(strings.ToLower(jenkinsServer), s3bucket) {
			log.Println("CB Server run url is already pointing to S3.")
			continue
		}
		//fmt.Println("Jenkins Server: ", jenkinsServer)
		jenkinsUser := props.MustGetString(jenkinsServer + "_JENKINS_USER")
		jenkinsUserPwd := props.MustGetString(jenkinsServer + "_JENKINS_TOKEN")

		includedFiles := strings.Split(includes, ",")
		for i := 0; i < len(includedFiles); i++ {
			switch strings.ToLower(strings.TrimSpace(includedFiles[i])) {
			case "console":
				log.Println("...downloading console file")
				DownloadFileWithBasicAuth(LogFile, data.JobURL+data.BuildID+"/consoleText", jenkinsUser, jenkinsUserPwd)
				break
			case "config":
				log.Println("...downloading config file")
				DownloadFileWithBasicAuth(ConfigFile, data.JobURL+"/config.xml", jenkinsUser, jenkinsUserPwd)
				break
			case "parameters":
				log.Println("...downloading parameters file")
				DownloadFileWithBasicAuth(JobFile, data.JobURL+data.BuildID+"/api/json?pretty=true", jenkinsUser, jenkinsUserPwd)
				break
			case "testresult":
				log.Println("...downloading testresult file")
				DownloadFileWithBasicAuth(ResultFile, data.JobURL+data.BuildID+"/testReport/api/json?pretty=true", jenkinsUser, jenkinsUserPwd)
				DownloadFileWithBasicAuth(ResultFileXML, data.JobURL+data.BuildID+"/testReport/api/xml?pretty=true", jenkinsUser, jenkinsUserPwd)
				break
			case "archive":
				log.Println("...downloading archive file")
				DownloadFileWithBasicAuth(ArchiveZipFile, data.JobURL+data.BuildID+"/artifact/*zip*/archive.zip", jenkinsUser, jenkinsUserPwd)
				break
			}
		}

		// Create index.html file
		indexFile := JobDir + "/" + "index.html"
		index, _ := os.Create(indexFile)
		defer index.Close()

		indexBuffer := bufio.NewWriter(index)
		fmt.Fprintf(indexBuffer, "<h1>CB Server build: %s</h1>\n<ul>", cbbuild)
		fmt.Fprintf(indexBuffer, "<h2>OS: %s</h2>\n<ul>", cbplatform)
		fmt.Fprintf(indexBuffer, "<h3>Test Suite: %s</h3>\n<ul>", data.TestName)
		// Save in AWS S3
		if strings.Contains(dest, "s3") {
			log.Println("Saving to S3 ...")
			//SaveInAwsS3(ConfigFile)
			for i := 0; i < len(includedFiles); i++ {
				switch strings.ToLower(strings.TrimSpace(includedFiles[i])) {
				case "console":
					if fileExists(LogFile) {
						SaveInAwsS3(LogFile)
						fmt.Fprintf(indexBuffer, "\n<li><a href=\"consoleText.txt\" target=\"_blank\">Jenkins job console log</a>")
					}
					break
				case "testresult":
					if fileExists(ResultFile) {
						SaveInAwsS3(ResultFile)
						fmt.Fprintf(indexBuffer, "\n<li><a href=\"testresult.json\" target=\"_blank\">Test result json</a>")
					}
					if fileExists(ResultFileXML) {
						SaveInAwsS3(ResultFileXML)
						fmt.Fprintf(indexBuffer, "\n<li><a href=\"testresult.xml\" target=\"_blank\">Test result XML</a>")
					}
					break
				case "config":
					if fileExists(ConfigFile) {
						SaveInAwsS3(ConfigFile)
						fmt.Fprintf(indexBuffer, "\n<li><a href=\"config.xml\" target=\"_blank\">Jenkins job config</a>")
					}
					break
				case "parameters":
					if fileExists(JobFile) {
						SaveInAwsS3(JobFile)
						fmt.Fprintf(indexBuffer, "\n<li><a href=\"jobinfo.json\" target=\"_blank\">Jenkins job parameters</a>")
					}
					break
				case "archive":
					if fileExists(ArchiveZipFile) {
						SaveInAwsS3(ArchiveZipFile)
						fmt.Fprintf(indexBuffer, "\n<li><a href=\"archive.zip\" target=\"_blank\">Jenkins artifacts archive zip</a>")
					}
					break
				}
			}
			fmt.Fprintf(indexBuffer, "\n</ul>")
			//SaveInAwsS3(ConfigFile, JobFile, ResultFile, LogFile)
			indexBuffer.Flush()

			if fileExists(indexFile) {
				SaveInAwsS3(indexFile)
				log.Println("Original URL: " + data.JobURL + data.BuildID + "/")
				log.Println("S3 URL: http://" + s3bucket + ".s3-website-us-west-2.amazonaws.com/" +
					cbbuild + "/" + "jenkins_logs" + "/" + JobName + "/" + data.BuildID + "/")
				// Update URL in CB server
				if strings.Contains(strings.ToLower(updateURL), "yes") && !strings.Contains(strings.ToLower(data.JobURL), s3bucket) {
					qry := "update `server` set url='http://" + s3bucket + ".s3-website-us-west-2.amazonaws.com/" +
						cbbuild + "/" + "jenkins_logs" + "/" + JobName + "/' where `build`='" +
						cbbuild + "' and url like '%/" + JobName + "/' and  build_id=" + data.BuildID
					fmt.Println("CB update in progress with qry= " + qry)
					if err := executeN1QLPostStmt(url, qry); err != nil {
						panic(err)
					}
				}
			}
		}

	}

}

// DownloadFromJenkins ...
func DownloadFromJenkins(outFileName string, fromURL string) {
	URLParts := strings.Split(fromURL, "/")
	jenkinsServer := strings.ToUpper(strings.Split(URLParts[2], ".")[0])

	//fmt.Println("Jenkins Server: ", jenkinsServer)
	props := properties.MustLoadFile("${HOME}/.jenkins_env.properties", properties.UTF8)
	jenkinsUser := props.MustGetString(jenkinsServer + "_JENKINS_USER")
	jenkinsUserPwd := props.MustGetString(jenkinsServer + "_JENKINS_TOKEN")

	DownloadFileWithBasicAuth(outFileName, fromURL, jenkinsUser, jenkinsUserPwd)
}

// JenkinsBuildResult type
type JenkinsBuildResult struct {
	Actions     []JenkinsActions
	Description string
	Result      string
}

// JenkinsActions type
type JenkinsActions struct {
	Parameters []JenkinsParameters
}

// JenkinsParameters type
type JenkinsParameters struct {
	Name  string
	Value interface{}
}

//GetJenkinsLastBuildFromDesc - Get the last jenkins build number
func GetJenkinsLastBuildFromDesc(buildURL string) string {
	jenkinsLastBuildFile := "jenkins_lastbuild.json"
	DownloadFromJenkins(jenkinsLastBuildFile, buildURL)
	resultFile, err := os.Open(jenkinsLastBuildFile)
	if err != nil {
		fmt.Println(err)
	}
	defer resultFile.Close()

	byteValue, _ := ioutil.ReadAll(resultFile)

	var result JenkinsBuildResult

	err = json.Unmarshal(byteValue, &result)
	cbbuildFromJenkins := ""
	if err == nil {
		if "SUCCESS" == result.Result {
			cbbuildFromJenkins = result.Description
		}
	} else {
		cbbuildFromJenkins = ""
		fmt.Println(err)
	}
	return strings.TrimSpace(cbbuildFromJenkins)
}

// JenkinsBuildInfo type
type JenkinsBuildInfo struct {
	Building          bool
	Description       string
	DisplayName       string
	Duration          int64
	EstimatedDuration int64
	Executor          string
	FullDisplayName   string
	ID                string
	KeepLog           bool
	Number            int
	QueueID           int64
	Result            string
	Timestamp         int64
	URL               string
	BuiltOn           string
}

//DownloadJenkinsURLContent for given URL
func DownloadJenkinsURLContent(fromURL string) []byte {
	//DownloadFromJenkins(outFile, buildURL)

	URLParts := strings.Split(fromURL, "/")
	jenkinsServer := strings.ToUpper(strings.Split(URLParts[2], ".")[0])

	//fmt.Println("Jenkins Server: ", jenkinsServer)
	props := properties.MustLoadFile("${HOME}/.jenkins_env.properties", properties.UTF8)
	jenkinsUser := props.MustGetString(jenkinsServer + "_JENKINS_USER")
	jenkinsUserPwd := props.MustGetString(jenkinsServer + "_JENKINS_TOKEN")

	byteValue, _ := DownloadFileWithBasicAuthContent(fromURL, jenkinsUser, jenkinsUserPwd)

	return byteValue
}

// DownloadFileWithBasicAuthContent will download the given url and return content.
func DownloadFileWithBasicAuthContent(remoteURL string, userName string, pwd string) ([]byte, error) {
	//fmt.Println("Downloading ...", localFilePath, "--", remoteURL, "---", userName, "---", pwd)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", remoteURL, nil)
	//localFilePath := path.Base(req.URL.Path)
	req.SetBasicAuth(userName, pwd)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		//log.Println("Warning: ", remoteURL, " not found!")
		return nil, err
	}
	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	return responseData, err

}

// TestSuiteN1QLQryResult type
type TestSuiteN1QLQryResult struct {
	Status  string
	Results []TestSuites
}

// TestSuites type
type TestSuites struct {
	TotalSuiteCount int
}

//GenSummaryForRunProgress ...
//func GenSummaryForRunProgress(filename string, cbbuild string) {
func GenSummaryForRunProgress(cbbuild string) {

	fmt.Println("Generating summary for run progress ...")
	servercburl := url
	//serverpoolcburl := "http://172.23.105.177:8093/query/service"

	/*
		//read triggered urls from file
		f1, err1 := os.Open(filename)
		if err1 != nil {
			log.Println(err1)
		}
		defer f1.Close()

		// Splits on newlines by default.
		scanner := bufio.NewReader(f1)

		line := 0
		linestring := ""

		totalExpectedSuites := 0

		f, _ := os.Create("component_testsuites.txt")
		defer f.Close()
		sno := 1
		w := bufio.NewWriter(f)
		// https://golang.org/pkg/bufio/#Scanner.Scan
		for {
			linestring, err1 = scanner.ReadString('\n')
			if err1 != nil {
				f1.Close()
				break
			}
			linestring1 := strings.ReplaceAll(strings.ReplaceAll(linestring, "$", ""), "\n", "")
			//fmt.Println("url=" + linestring1)
			u, err2 := gourl.Parse(linestring1)
			if err2 != nil {
				fmt.Println(err2)
			}
			m, _ := gourl.ParseQuery(u.RawQuery)
			suiteType := ""
			if m["suite"] != nil {
				suiteType = m["suite"][0]
			}
			//else {
			//	suiteType = defaultSuiteType
			//}
			components := ""
			if m["component"] != nil {
				component1 := strings.Split(m["component"][0], ",")
				for i := 0; i < len(component1); i++ {
					components += "\"" + strings.TrimSpace(component1[i]) + "\","
				}
				index1 := strings.LastIndex(components, ",")
				components = components[:index1]
			}
			subcomponents := ""
			if m["subcomponent"] != nil {
				subcomponent1 := strings.Split(m["subcomponent"][0], ",")
				for i := 0; i < len(subcomponent1); i++ {
					subcomponents += "\"" + strings.TrimSpace(subcomponent1[i]) + "\","
				}
				index1 := strings.LastIndex(subcomponents, ",")
				subcomponents = subcomponents[:index1]
			}

			//run cb query
			qry := ""
			if suiteType != "" {
				if components != "" && subcomponents != "" {
					qry = "select count(*) as TotalSuiteCount from `QE-Test-Suites` where \"" + suiteType + "\" in partOf and component in [" + components + "] and subcomponent in [" + subcomponents + "]"
				} else if components != "" {
					qry = "select count(*) as TotalSuiteCount from `QE-Test-Suites` where \"" + suiteType + "\" in partOf and component in [" + components + "]"
				} else {
					continue //skip for now if no component
				}
			} else {
				if components != "" && subcomponents != "" {
					qry = "select count(*) as TotalSuiteCount from `QE-Test-Suites` where component in [" + components + "] and subcomponent in [" + subcomponents + "]"
				} else if components != "" {
					qry = "select count(*) as TotalSuiteCount from `QE-Test-Suites` where component in [" + components + "]"
				} else {
					continue //skip for now if no component
				}
			}

			//fmt.Println("query=" + qry)
			localFileName := "suiteresult.json"
			if err := executeN1QLStmt(localFileName, serverpoolcburl, qry); err != nil {
				panic(err)
			}

			resultFile, err := os.Open(localFileName)
			if err != nil {
				fmt.Println(err)
			}
			defer resultFile.Close()

			byteValue, _ := ioutil.ReadAll(resultFile)

			var result TestSuiteN1QLQryResult

			err = json.Unmarshal(byteValue, &result)
			//fmt.Println("Status=" + result.Status)
			//fmt.Println(err)
			if result.Status == "success" {
				//fmt.Println("Count: ", len(result.Results))
				fmt.Fprintf(w, "\n%s\t %s\t%3d\n", components, subcomponents, result.Results[0].TotalSuiteCount)
			} else {
				fmt.Println("CB Query failed!")
			}

			totalExpectedSuites += result.Results[0].TotalSuiteCount

			line++
		}*/

	// 1. Get total number of jobs: unique number of jobs across the release.
	cbrelease = strings.Split(cbbuild, "-")[0]
	jtqry := "select count(distinct name) as TotalNumofjobs from `server` " +
		"where lower(os) like \"" + cbplatform + "\" and `build` like \"" + cbrelease + "%\""
	//fmt.Println("\nquery=" + jtqry)
	//fmt.Println("\nurl=" + url)
	jtlocalFileName := "totaluniqjobs_" + cbrelease + ".json"
	if jterr := executeN1QLStmt(jtlocalFileName, servercburl, jtqry); jterr != nil {
		//panic(err)
		log.Println(jterr)
	}

	jtresultFile, jterr := os.Open(jtlocalFileName)
	if jterr != nil {
		fmt.Println(jterr)
	}
	defer jtresultFile.Close()
	jtbyteValue, _ := ioutil.ReadAll(jtresultFile)
	var jtresult TotalJobsQryResult
	jterr = json.Unmarshal(jtbyteValue, &jtresult)
	totalReleasejobs := 0
	if jtresult.Status == "success" {
		totalReleasejobs = jtresult.Results[0].TotalNumofjobs
	}

	//2. Get number of completed and pending jobs
	jqry := "select `build`, numofjobs, totaltime, failcount, totalcount from (select b.`build`, count(*) as numofjobs, sum(duration) as totaltime, sum(failCount) as failcount, sum(totalCount) as totalcount from " + cbbucket + " b " +
		"where lower(b.os) like \"" + cbplatform + "\" and b.`build` like \"" + cbbuild + "%\" group by b.`build` order by b.`build` desc) as result " + qryfilter + " limit " + limits
	//fmt.Println("\nquery=" + jqry)
	//fmt.Println("\nurl=" + url)
	jlocalFileName := "buildprogressdetails.json"
	if jerr := executeN1QLStmt(jlocalFileName, servercburl, jqry); jerr != nil {
		//panic(err)
		log.Println(jerr)
	}
	jresultFile, jerr := os.Open(jlocalFileName)
	if jerr != nil {
		fmt.Println(jerr)
	}
	defer jresultFile.Close()

	jbyteValue, _ := ioutil.ReadAll(jresultFile)

	var jresult TotalCycleTimeQryResult
	var numofjobs int
	var abortedJobs, failureJobs, unstableJobs, successJobs int
	var passCount, failCount, totalCount int
	var hours, mins float64
	var secs int64
	//var totalTime int64
	jerr = json.Unmarshal(jbyteValue, &jresult)
	if jresult.Status == "success" {
		//fmt.Println(" Total time in millis: ", jresult.Results[0].Totaltime)
		totalhours := 0
		for i := 0; i < len(jresult.Results); i++ {
			cbbuild = jresult.Results[i].Build

			// get jobs status
			abortedJobs, failureJobs, unstableJobs, successJobs = getJobsStatusList(cbbuild)

			hours = math.Floor(float64(jresult.Results[i].Totaltime) / 1000 / 60 / 60)
			totalhours += int(hours)
			secs = jresult.Results[i].Totaltime % (1000 * 60 * 60)
			mins = math.Floor(float64(secs) / 60 / 1000)
			//secs = result.Results[i].Totaltime * 1000 % 60
			passCount = jresult.Results[i].Totalcount - jresult.Results[i].Failcount
			numofjobs = jresult.Results[i].Numofjobs
			totalCount = jresult.Results[i].Totalcount
			failCount = jresult.Results[i].Failcount
			//totalTime = jresult.Results[i].Totaltime
		}
	}
	queuedJobs := totalReleasejobs - numofjobs
	currentTime := time.Now().Format(time.RFC3339)

	//3. Get the slaves information
	props := properties.MustLoadFile("${HOME}/.jenkins_env.properties", properties.UTF8)
	jenkinsServer := "QA"
	jenkinsUser := props.MustGetString(jenkinsServer + "_JENKINS_USER")
	jenkinsUserPwd := props.MustGetString(jenkinsServer + "_JENKINS_TOKEN")
	jenkinsSlavesURL := qaJenkinsURL + "computer/api/json?pretty=true"
	slavesFile := "jenkins_slaves.json"
	DownloadFileWithBasicAuth(slavesFile, jenkinsSlavesURL, jenkinsUser, jenkinsUserPwd)
	jeresultFile, jeerr := os.Open(slavesFile)
	if jeerr != nil {
		fmt.Println(jeerr)
	}
	defer jeresultFile.Close()
	jebyteValue, _ := ioutil.ReadAll(jeresultFile)
	var jeresult JenkinsComputerQryResult
	jeerr = json.Unmarshal(jebyteValue, &jeresult)
	totalNumofSlaves := len(jeresult.Computer) - 1
	//fmt.Printf("\nslaves total executors=%d\tbusy executors=%d\tslaves count=%d", jeresult.TotalExecutors, jeresult.BusyExecutors, totalNumofSlaves)

	p0SlavesLabels := "P0"
	numberofP0Slaves := 0
	numberofP0Executors := 0
	numberofP0offline := 0
	numberofP0available := 0
	numberofP0availableExecutors := 0
	numberofP0offlineExecutors := 0

	for i := 0; i < totalNumofSlaves; i++ {
		for j := 0; j < len(jeresult.Computer[i].AssignedLabels); j++ {
			if strings.Contains(jeresult.Computer[i].AssignedLabels[j].Name, p0SlavesLabels) {
				numberofP0Slaves++
				numberofP0Executors += jeresult.Computer[i].NumExecutors
				if jeresult.Computer[i].Offline || jeresult.Computer[i].temporarilyOffline {
					numberofP0offline++
					numberofP0offlineExecutors += jeresult.Computer[i].NumExecutors
				} else {
					numberofP0available++
					numberofP0availableExecutors += jeresult.Computer[i].NumExecutors
				}
				break

			}
		}
		//fmt.Printf("\n%s\t%t\t%t\t%d", jeresult.Computer[i].DisplayName, jeresult.Computer[i].Offline, jeresult.Computer[i].temporarilyOffline,
		//	jeresult.Computer[i].NumExecutors)

	}

	//fmt.Printf("\n%3d\t%3d\t%3d\t%3d\t%3d\t%3d", numberofP0Slaves, numberofP0Executors, numberofP0offline, numberofP0available, numberofP0availableExecutors, numberofP0offlineExecutors)

	//4. Get server VMs for the required pools
	vmsCount := GetServerPoolVMs(cbplatform, requiredServerPools, requiredStates)

	//5. Print Summary
	//fmt.Fprintf(w, "\nTotal #of Jobs Kicked off: %3d\n", totalExpectedSuites)
	sno := 1
	reportOutputFileName := "summary_progress_" + cbbuild + ".txt"
	//userHome, _ := os.UserHomeDir()
	//snoFile := userHome + string(os.PathSeparator) + ".sno_" + cbbuild + ".txt"
	f, err := os.OpenFile(reportOutputFileName, os.O_APPEND|os.O_WRONLY, 0600)
	isNewFile := false
	if err != nil {
		f, _ = os.Create(reportOutputFileName)
		isNewFile = true
		sno = 1
		//writeContent(snoFile, strconv.Itoa(sno))
	} else {
		// read last record to get the sno
		//ssno, _ := readContent(snoFile)
		//sno, _ = strconv.Atoi(strings.TrimSpace(ssno))
		//fmt.Printf("ssno=%s,sno=%d", ssno, sno)
		//sno++

		lastsnostr, _ := readTailN(reportOutputFileName, 2)
		if lastsnostr != "" {
			ssno := strings.TrimSpace(strings.Split(lastsnostr, ".")[0])
			sno, _ = strconv.Atoi(strings.TrimSpace(ssno))
			//fmt.Printf("lastsnostr=%s,ssno=%s,sno=%d", lastsnostr, ssno, sno)
			sno++
		}

	}
	defer f.Close()
	w := bufio.NewWriter(f)

	//fmt.Printf("\n*** Test execution progress summary report ***\n Build: %s\n Server pools:%s", cbbuild, requiredServerPools)
	fmt.Printf("\n*** Test execution progress summary report for build#%s on %s ***", cbbuild, cbplatform)
	fmt.Printf("\n-------------------------------------------------------------------------------------------------------------------" +
		"--------------------------------------------------------------------------------------------------------------")
	//fmt.Printf("\nS.No.\tTimestamp\t\t#ofJobsKickedoff\t#ofJobsCompleted(A,F,U,S)\t#ofJobsQueued\t#ofP0SlavesAvailable(E)\t#ofSlavesUsed(E)\t" +
	//	"#ofServerVMsAvailable\t#ofServerVMsUsed\t#ofTestsExecuted\t#Passed\t#Failed \tPassRate\tTotaltime")
	fmt.Printf("\nS.No.\tTimestamp\t\t\t#of Jobs\t#of Jobs\t\t#of Jobs\t#ofTests\t#Passed\t#Failed PassRate Totaltime\t\t#ofP0Slaves\t#ofSlaves\t" +
		"#ofServerVMs\t#ofServerVMs")
	fmt.Printf("\n\t\t\t\t\tKickedoff\tCompleted(A,F,U,S)\t#Queued\t\tExecuted\t\t\t\t\t\t\tAvailable(E)\tUsed(E)\t\t" +
		"Available\tUsed")
	fmt.Printf("\n-------------------------------------------------------------------------------------------------------------------" +
		"--------------------------------------------------------------------------------------------------------------")

	fmt.Printf("\n%2d.\t%s\t%3d\t\t"+
		"%3d(%3d,%3d,%3d,%3d)\t%3d\t\t"+
		"%5d\t\t%5d\t%5d\t"+
		"%6.2f%%\t"+
		"%4d hrs %2d mins\t"+
		"%3d/%3d(%3d/%3d) %s/%3d(%3d/%3d)\t"+
		"%3d\t\t%3d\t\t",
		sno, currentTime, totalReleasejobs,
		numofjobs, abortedJobs, failureJobs, unstableJobs, successJobs, queuedJobs,
		totalCount, passCount, failCount,
		(float32(passCount)/float32(totalCount))*100,
		int64(hours), int64(mins),
		numberofP0available, numberofP0Slaves, numberofP0availableExecutors, numberofP0Executors, "-", totalNumofSlaves, jeresult.BusyExecutors, jeresult.TotalExecutors,
		vmsCount[0], vmsCount[1])
	fmt.Printf("\n-------------------------------------------------------------------------------------------------------------------" +
		"--------------------------------------------------------------------------------------------------------------\n")
	// 4.2. save in the file
	if isNewFile {
		fmt.Fprintf(w, "\n*** Test execution progress summary report for build#%s on %s ***", cbbuild, cbplatform)
		fmt.Fprintf(w, "\n-------------------------------------------------------------------------------------------------------------------"+
			"--------------------------------------------------------------------------------------------------------------")
		fmt.Fprintf(w, "\nS.No.\tTimestamp\t\t\t#of Jobs\t#of Jobs\t\t#of Jobs\t#ofTests\t#Passed\t#Failed PassRate Totaltime\t\t#ofP0Slaves\t#ofSlaves\t"+
			"#ofServerVMs\t#ofServerVMs")
		fmt.Fprintf(w, "\n\t\t\t\t\tKickedoff\tCompleted(A,F,U,S)\t#Queued\t\tExecuted\t\t\t\t\t\t\tAvailable(E)\tUsed(E)\t\t"+
			"Available\tUsed")
		fmt.Fprintf(w, "\n-------------------------------------------------------------------------------------------------------------------"+
			"--------------------------------------------------------------------------------------------------------------")
	}
	fmt.Fprintf(w, "\n%2d.\t%s\t%3d\t\t"+
		"%3d(%3d,%3d,%3d,%3d)\t%3d\t\t"+
		"%5d\t\t%5d\t%5d\t"+
		"%6.2f%%\t"+
		"%4d hrs %2d mins\t"+
		"%3d/%3d(%3d/%3d) %s/%3d(%3d/%3d)\t"+
		"%3d\t\t%3d\t\t",
		sno, currentTime, totalReleasejobs,
		numofjobs, abortedJobs, failureJobs, unstableJobs, successJobs, queuedJobs,
		totalCount, passCount, failCount,
		(float32(passCount)/float32(totalCount))*100,
		int64(hours), int64(mins),
		numberofP0available, numberofP0Slaves, numberofP0availableExecutors, numberofP0Executors, "-", totalNumofSlaves, jeresult.BusyExecutors, jeresult.TotalExecutors,
		vmsCount[0], vmsCount[1])
	fmt.Fprintf(w, "\n-------------------------------------------------------------------------------------------------------------------"+
		"--------------------------------------------------------------------------------------------------------------\n")
	w.Flush()
	f.Close()
	//writeContent(snoFile, strconv.Itoa(sno))
	fmt.Println("NOTE: Please check the final progress summary report file at " + f.Name())
}

// PoolN1QLQryResult type
type PoolN1QLQryResult struct {
	Status  string
	Results []QEServerPool
}

// QEServerPool type
type QEServerPool struct {
	IPaddr  string
	Origin  string
	HostOS  string
	SpoolID string
	PoolID  []string
	State   string
}

// HostOSN1QLQryResult type
type HostOSN1QLQryResult struct {
	Status  string
	Results []HostOSCount
}

// HostOSCount type
type HostOSCount struct {
	HostOS string
	Count  int16
}

//GetServerPoolVMs ...
func GetServerPoolVMs(osplatform string, reqserverpools string, reqstates string) []int {

	url := "http://172.23.105.177:8093/query/service"
	//osplatform := "centos"
	qry := "select ipaddr,origin,os as hostos,poolId as spoolId, poolId,state from `QE-server-pool` where lower(`os`)='" + osplatform + "'"
	//fmt.Println("query=" + qry)
	localFileName := "result.json"
	if err := executeN1QLStmt(localFileName, url, qry); err != nil {
		panic(err)
	}

	resultFile, err := os.Open(localFileName)
	if err != nil {
		fmt.Println(err)
	}
	defer resultFile.Close()

	byteValue, _ := ioutil.ReadAll(resultFile)
	var result PoolN1QLQryResult
	err = json.Unmarshal(byteValue, &result)
	var counts []int
	counts = make([]int, 0)
	if result.Status == "success" {
		//fmt.Println("Count: ", len(result.Results))
		var pools map[string]string
		pools = make(map[string]string)
		var states map[string]string
		states = make(map[string]string)
		var poolswithstates map[string]string
		poolswithstates = make(map[string]string)
		var vms map[string]string
		vms = make(map[string]string)

		for i := 0; i < len(result.Results); i++ {
			//fmt.Println((i + 1), result.Results[i].Aname, result.Results[i].JURL, result.Results[i].URLbuild)
			//fmt.Print(result.Results[i].IPaddr, ", ", result.Results[i].HostOS, ", ", result.Results[i].State, ", [")
			//for j := 0; j < len(result.Results[i].PoolID); j++ {
			//	fmt.Print(result.Results[i].PoolID[j], ", ")
			//}
			//fmt.Println("]")
			//_, err = fmt.Fprintf(w, "%s,%s,%s\n", result.Results[i].IPaddr,
			//	result.Results[i].HostOS, result.Results[i].State)

			//pools level
			if result.Results[i].SpoolID != "" {
				//pools[result.Results[i].SpoolID+result.Results[i].HostOS] = pools[result.Results[i].SpoolID+result.Results[i].HostOS] + result.Results[i].IPaddr + "\n"
				pools[result.Results[i].SpoolID] = pools[result.Results[i].SpoolID] + result.Results[i].IPaddr + "\n"
				poolswithstates[result.Results[i].SpoolID+result.Results[i].State] = poolswithstates[result.Results[i].SpoolID+result.Results[i].State] + result.Results[i].IPaddr + "\n"
				vms[result.Results[i].IPaddr] = vms[result.Results[i].IPaddr] + result.Results[i].SpoolID + ","

				//fmt.Println("result.Results[i].SpoolID=", result.Results[i].SpoolID+", PoolID length=", len(result.Results[i].PoolID))
			} else {
				for j := 0; j < len(result.Results[i].PoolID); j++ {
					if !strings.Contains(result.Results[i].IPaddr, "[f") {
						pools[result.Results[i].PoolID[j]] = pools[result.Results[i].PoolID[j]] + result.Results[i].IPaddr + "\n"
						poolswithstates[result.Results[i].PoolID[j]+result.Results[i].State] = poolswithstates[result.Results[i].PoolID[j]+result.Results[i].State] + result.Results[i].IPaddr + "\n"
						vms[result.Results[i].IPaddr] = vms[result.Results[i].IPaddr] + result.Results[i].PoolID[j] + ","
					} else {
						pools[result.Results[i].PoolID[j]] = pools[result.Results[i].PoolID[j]] + "#" + result.Results[i].IPaddr + "\n"
						poolswithstates[result.Results[i].PoolID[j]+result.Results[i].State] = poolswithstates[result.Results[i].PoolID[j]+result.Results[i].State] + "#" + result.Results[i].IPaddr + "\n"
						vms[result.Results[i].IPaddr] = vms[result.Results[i].IPaddr] + result.Results[i].PoolID[j] + ","
					}
					//_, err = fmt.Fprintf(w, ",%s", result.Results[i].PoolID[j])
					//fmt.Println("result.Results[i].SpoolID=", result.Results[i].SpoolID+", PoolID length=", len(result.Results[i].PoolID))
				}

			}
			vms[result.Results[i].IPaddr] = vms[result.Results[i].IPaddr] + result.Results[i].State

			// states level
			if !strings.Contains(result.Results[i].IPaddr, "[f") {
				states[result.Results[i].State] = states[result.Results[i].State] + result.Results[i].IPaddr + "\n"
			} else {
				states[result.Results[i].State] = states[result.Results[i].State] + "#" + result.Results[i].IPaddr + "\n"
			}

		}
		//summary and generation of .ini - write to file

		//fmt.Println("\nBy Pools with State")
		//fmt.Println("---------------------")
		f2, err2 := os.Create("vmpoolswithstates_" + osplatform + "_ips.ini")
		if err2 != nil {
			log.Println(err2)
		}
		defer f2.Close()
		w2 := bufio.NewWriter(f2)

		var pskeys []string
		for k := range poolswithstates {
			pskeys = append(pskeys, k)
		}
		sort.Strings(pskeys)
		totalHosts := 0
		psf, _ := os.Create("vmpoolswithstates_" + osplatform + "_counts.txt")
		defer psf.Close()
		psfw := bufio.NewWriter(psf)

		reqstates1 := strings.Split(reqstates, ",")
		reqserverpools1 := strings.Split(reqserverpools, ",")
		for i := 0; i < len(reqstates1); i++ {
			count := 0
			for j := 0; j < len(reqserverpools1); j++ {
				k := reqserverpools1[j] + reqstates1[i]
				count += len(strings.Split(poolswithstates[k], "\n")) - 1
				//fmt.Printf("reqserverpools[j]=%s,reqstates[i]=%s,k=%s, count=%d", reqserverpools1[j], reqstates1[i], k, count)
			}

			counts = append(counts, count)
		}
		for _, k := range pskeys {
			nk := strings.ReplaceAll(k, " ", "")
			nk = strings.ReplaceAll(nk, "-", "")
			_, err = fmt.Fprintf(w2, "\n[%s]\n%s", nk, poolswithstates[k])
			count := len(strings.Split(poolswithstates[k], "\n")) - 1
			totalHosts += count
			//fmt.Printf("%s: %d\n", nk, count)
			fmt.Fprintf(psfw, "%s: %d\n", nk, count)
		}
		//fmt.Println("\n Total: ", totalHosts)
		w2.Flush()
		psfw.Flush()

	} else {
		fmt.Println("CBQuery Status: Failed")
	}
	return counts
}

// fileExists ...
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// SaveInAwsS3 ...
func SaveInAwsS3(files ...string) {
	for i := 0; i < len(files); i++ {
		if overwrite == "no" {
			cmd1 := "aws s3 ls " + "s3://" + s3bucket + "/" + files[i]
			//fmt.Println(cmd1)
			cmd1Out := executeCommand(cmd1, "")
			//fmt.Println(cmd1, "--"+cmd1Out)
			fileParts := strings.Split(files[i], "/")
			fileName := fileParts[len(fileParts)-1]
			//fmt.Println("fileName=", fileName)
			if strings.Contains(cmd1Out, fileName) && overwrite == "no" {
				log.Println("Warning: Upload skip as AWS S3 already contains " + files[i] + " and overwrite=no")
			} else {
				SaveFileToS3(files[i])
			}
		} else {
			SaveFileToS3(files[i])
		}
	}
}

// SaveFileToS3 ...
func SaveFileToS3(objectName string) {
	cmd := "aws s3 cp " + objectName + " s3://" + s3bucket + "/" + objectName
	//fmt.Println("cmd=", cmd)
	outmesg := executeCommand(cmd, "")
	if outmesg != "" {
		log.Println(outmesg)
	}
	// read permission - only needed if bucket policy is not there
	//cmd = "aws s3api put-object-acl --bucket " + s3bucket + " --key " + objectName + " --acl public-read "
	//fmt.Println("cmd=", cmd)
	//outmesg = executeCommand(cmd, "")
	//if outmesg != "" {
	//	log.Println(outmesg)
	//}
}

// ReadCsv ... read csv file as double dimension array
func ReadCsv(filename string) ([][]string, error) {
	// Open CSV file
	f, err := os.Open(filename)
	if err != nil {
		return [][]string{}, err
	}
	defer f.Close()

	// Read File into a Variable
	lines, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return [][]string{}, err
	}
	f.Close()
	return lines, nil
}

//SearchFile ...
func SearchFile(filename string, substring string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Splits on newlines by default.
	scanner := bufio.NewScanner(f)

	line := 1
	linestring := ""
	// https://golang.org/pkg/bufio/#Scanner.Scan
	for scanner.Scan() {
		linestring = scanner.Text()
		if strings.Contains(linestring, substring) {
			f.Close()
			return linestring, nil
		}
		line++
	}

	if err := scanner.Err(); err != nil {
		log.Println(err)
		return "", err
	}
	f.Close()
	return linestring, err
}

//SearchFile2 ...
func SearchFile2(filename string, substring string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Splits on newlines by default.
	scanner := bufio.NewReader(f)

	line := 1
	linestring := ""
	// https://golang.org/pkg/bufio/#Scanner.Scan
	for {
		linestring, err = scanner.ReadString('\n')
		if err != nil {
			f.Close()
			return "", err
		}
		if strings.Contains(linestring, substring) {
			f.Close()
			return linestring, nil
		}
		line++
	}
}

//SearchFileNextLines ...
func SearchFileNextLines(filename string, substring string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Splits on newlines by default.
	scanner := bufio.NewScanner(f)

	line := 1
	linestring := ""
	// https://golang.org/pkg/bufio/#Scanner.Scan
	for scanner.Scan() {
		linestring = scanner.Text()
		if strings.Contains(linestring, substring) {
			scanner.Scan()
			linestring = scanner.Text()
			lines := linestring
			for linestring != "\n" && linestring != "" {
				lines += linestring + "\n"
				if scanner.Scan() {
					linestring = scanner.Text()
					log.Println(linestring)
				} else {
					break
				}
			}
			f.Close()
			return lines, nil
		}
		line++
	}

	if err := scanner.Err(); err != nil {
		log.Println(err)
		return "", err
	}
	return linestring, err
}

//SearchFileNextLines2 ...
func SearchFileNextLines2(filename string, substring string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Splits on newlines by default.
	scanner := bufio.NewReader(f)

	line := 1
	linestring := ""
	// https://golang.org/pkg/bufio/#Scanner.Scan
	for {
		linestring, err = scanner.ReadString('\n')
		if err != nil {
			return "", nil
		}
		if strings.Contains(linestring, substring) {
			linestring, err = scanner.ReadString('\n')
			lines := ""
			for linestring != "\n" && linestring != "" && err != io.EOF {
				lines += linestring
				linestring, err = scanner.ReadString('\n')
				if err == nil {
					//log.Println(linestring)
					continue
				} else {
					break
				}
			}
			f.Close()
			return lines, nil
		}
		line++
	}

}

//readContent ...
func readContent(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Splits on newlines by default.
	scanner := bufio.NewReader(f)

	//line := 1
	linestring := ""
	content := ""
	// https://golang.org/pkg/bufio/#Scanner.Scan
	for {
		linestring, err = scanner.ReadString('\n')
		if err != nil {
			f.Close()
			return "", err
		}
		content += linestring
		//line++
		return content, err
	}
}

//writeContent ...
func writeContent(filename string, content string) {
	outFile, _ := os.Create(filename)
	outW := bufio.NewWriter(outFile)
	defer outFile.Close()
	fmt.Fprintf(outW, "%s\n", content)
	outW.Flush()
	outFile.Close()
}

//readTailN ...
func readTailN(filename string, n int) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Splits on newlines by default.
	scanner := bufio.NewReader(f)

	line := 0
	linestring := ""

	var content []string
	content = make([]string, 0)
	// https://golang.org/pkg/bufio/#Scanner.Scan
	for {
		linestring, err = scanner.ReadString('\n')

		if err == io.EOF {
			f.Close()
			//fmt.Printf("Returning..last#%d - %s", (line - n), content[line-n])
			return content[line-n], err
		} else if err != nil {
			fmt.Println(err)
			return content[line-n], err
		}
		//fmt.Printf("Appending...line#%d - %s", line, linestring)
		content = append(content, linestring)
		line++
		//return content[line-n-1], err
	}
}
