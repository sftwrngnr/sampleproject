package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
)

const DefaultDataLoc string = "../../perf-testing-data/ec2-huge/run36"
const DefaultCSVFileName string = "testdata.csv"
const DirectoryFormat string = "%s/%s/data"

type ParamKey struct {
	Host     string
	ParrConn int
}

type ParamRef struct {
	Conn    int
	Rate    int
	Threads int
	Mem     string
	CpuWt   int
	CpuTm   int
}

type Data struct {
	ReqSec   float32 //Requests/s
	TransSec float32 //Transfer rate Kb/s
}

var ProcDir []string

func CheckForCSVFiles(procdir string) bool {
	myDirInfo, err := ioutil.ReadDir(procdir)
	if err != nil {
		fmt.Sprintf("Cannot open directory %s. Failed with error: %s\n", procdir, err.Error())
		return false
	}
	for _, file := range myDirInfo {
		if !file.IsDir() {
			if strings.Index(file.Name(), ".csv") != -1 {
				return true
			}
		}
	}
	return false
}

func GetHostList(procdir string) bool {
	var retval bool = false
	var ans string
	if CheckForCSVFiles(procdir) {
		fmt.Printf("CSV files already exist in %s\nProceed and overwrite? (Y/N)", procdir)
		fmt.Scanf("%s", &ans)
		if strings.EqualFold(ans, "n") {
			return false
		}
	}
	myDirInfo, err := ioutil.ReadDir(procdir)
	if err != nil {
		fmt.Printf("Cannot open directory %s. Failed with error: %s\n", procdir, err.Error())
		return false
	}
	for _, file := range myDirInfo {
		if file.IsDir() {
			ProcDir = append(ProcDir, file.Name())
			retval = true // At least one directory to process
		}
	}
	return retval
}

func IntArrInsert(inarr []int, insertidx int, ival int) []int {
	if len(inarr) == 0 || insertidx > len(inarr)-1 {
		return append(inarr, ival)
	}
	inarr = inarr[0:len(inarr)]
	copy(inarr[insertidx+1:], inarr[insertidx:])
	inarr[insertidx] = ival
	return inarr
}

func GetParallelServers(procdir string) ([]int, []string) {
	var parsrvlist []int
	var hostlist []string
	for _, ckDir := range ProcDir {
		DataDir := fmt.Sprintf(DirectoryFormat, procdir, ckDir)
		hostlist = append(hostlist, ckDir)
		myDirInfo, err := ioutil.ReadDir(DataDir)
		if err != nil {
			fmt.Printf("Cannot open directory %s. Failed with error: %s\n", myDirInfo, err.Error())
			continue
		}
		for _, file := range myDirInfo {
			if file.IsDir() {
				ifname, err := strconv.Atoi(file.Name())
				if err != nil {
					fmt.Printf("Error with integer conversion. %s", file.Name())
					continue
				}
				if parsrvlist == nil {
					parsrvlist = append(parsrvlist, ifname)
					continue
				}
				i := sort.Search(len(parsrvlist), func(i int) bool { return parsrvlist[i] >= ifname })
				if !((i < len(parsrvlist)) && (parsrvlist[i] == ifname)) {
					parsrvlist = IntArrInsert(parsrvlist, i, ifname)
				}
			}
		}
	}
	return parsrvlist, hostlist
}

func BuildParamsFromFile(fname string) (int, int, int, string, int, int) {
	var c, r, t, cn, ct int
	var m string
	var err error
	mystrarr := strings.Split(fname, "_")
	mytmparr := strings.Split(fname, ".")
	c, err = strconv.Atoi(mystrarr[1])
	if err != nil {
		fmt.Printf("Blew chow on conversion. %s\n", mystrarr[2])
	}
	r, err = strconv.Atoi(mystrarr[2])
	if err != nil {
		fmt.Printf("Blew chow on conversion. %s\n", mystrarr[2])
	}
	mystrarr = strings.Split(mystrarr[3], ".")
	t, err = strconv.Atoi(mystrarr[0])
	if err != nil {
		fmt.Printf("Blew chow on conversion. %s\n", mystrarr[0])
	}
	// Now get rest of params
	if len(mytmparr) == 2 {
		return c, r, t, "", -1, -1
	}
	//fmt.Printf("%v\n", mytmparr)
	mystrarr = strings.Split(mytmparr[1], "_")
	//fmt.Printf("%v\n", mystrarr)
	sindx := int(0)
	if len(mystrarr) == 2 {
		// First run set with memory values not specified
		m = ""

	} else {
		m = mystrarr[sindx]
		sindx = sindx + 1
	}
	cn, err = strconv.Atoi(mystrarr[sindx])
	if err != nil {
		fmt.Printf("Blew chow on conversion. %s\n", mystrarr[2])
	}
	sindx = sindx + 1
	ct, err = strconv.Atoi(mystrarr[sindx])
	if err != nil {
		fmt.Printf("Blew chow on conversion. %s\n", mystrarr[2])
	}
	// Handle 2 or 3 element params

	return c, r, t, m, cn, ct
}

func BuildTestParameterMap(procdir string, slist []int, hlist []string) map[ParamKey][]ParamRef {
	hdmap := make(map[ParamKey][]ParamRef)
	for _, hostdir := range hlist {
		for _, parsrv := range slist {
			mydir := fmt.Sprintf("%s/%d", fmt.Sprintf(DirectoryFormat, procdir, hostdir), parsrv)
			mydinf, _ := ioutil.ReadDir(mydir)
			// Check mydinf
			mapkey := ParamKey{hostdir, parsrv}
			for _, file := range mydinf {
				if !file.IsDir() {
					// BuildParamsFromFile
					c, r, t, m, cn, ct := BuildParamsFromFile(file.Name())
					if t > c {
						fmt.Printf("Error with threads(%d) > connections(%d) ignoring\n", t, c)
						continue
					}
					hdmap[mapkey] = append(hdmap[mapkey], ParamRef{c, r, t, m, cn, ct})
				}
			}

		}
	}
	return hdmap
}

func GetReqSec(intxt string) float32 {
	mystrs := strings.Split(intxt, ":")
	myval, err := strconv.ParseFloat(strings.Trim(mystrs[1], " "), 32)
	if err != nil {
		fmt.Printf("Blew chow in GetReqSec float conversion. %s\n", err.Error())
	}

	return float32(myval)
}

func GetScaledFromString(trate string) (float32, error) {
	// WE are going to scale data consistently to Kb/S
	var rate float64
	var err error
	rate, err = strconv.ParseFloat(trate[0:len(trate)-2], 32)
	if err != nil {
		fmt.Printf("Blew chow in GetScaledFromString(). %s\n", err.Error())
		return float32(rate), err
	}
	if strings.Contains(trate, "MB") {
		rate = rate * 1000.0 // Should this be 1024, because 1024 KB make up 1 MB
	}
	return float32(rate), err
}

func GetScaledTransferRate(intxt string) float32 {
	mystrs := strings.Split(intxt, ":")
	tTran := strings.Trim(mystrs[1], " ")
	myval, err := GetScaledFromString(tTran)
	if err != nil {
		fmt.Printf("Blew chow in GetScaledTransferRate float conversion. %s\n", err.Error())
	}
	return float32(myval)
}

func GetData(procdir string, hname string, parrconn int, pr ParamRef) Data {
	var retval Data
	var fname string
	if (pr.Mem == "") && (pr.CpuWt == -1) && (pr.CpuTm == -1) {
		fname = fmt.Sprintf("%s/%s/data/%d/results_%d_%d_%d.out", procdir, hname, parrconn, pr.Conn, pr.Rate, pr.Threads)
	} else if pr.Mem == "" {
		fname = fmt.Sprintf("%s/%s/data/%d/results_%d_%d_%d.%d_%d.out", procdir, hname, parrconn, pr.Conn, pr.Rate, pr.Threads, pr.CpuWt, pr.CpuTm)
	} else {
		fname = fmt.Sprintf("%s/%s/data/%d/results_%d_%d_%d.%s_%d_%d.out", procdir, hname, parrconn, pr.Conn, pr.Rate, pr.Threads, pr.Mem, pr.CpuWt, pr.CpuTm)
	}
	file, err := os.Open(fname)
	if err != err {
		fmt.Printf("Couldn't read %s\n", fname)
		return retval
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		txt := scanner.Text()
		if strings.Contains(txt, "Requests/sec") {
			retval.ReqSec = GetReqSec(txt)
		}
		if strings.Contains(txt, "Transfer/sec") {
			retval.TransSec = GetScaledTransferRate(txt)
		}

	}
	return retval
}

func ProcDataFile(procdir string, hname string, parrconn int, pref ParamRef) Data {
	var retval Data
	//fmt.Printf("Directory is %s/%d/\n", fmt.Sprintf(DirectoryFormat, procdir, hname), parrconn)
	retval = GetData(procdir, hname, parrconn, pref)

	//fmt.Printf("retval is %v\n", retval)
	return retval
}

func CheckDataDir(procdir string, host string, parrconn int) bool {
	//dirname := fmt.Sprintf("%s/%s/data/%d", procdir, host, parrconn)
	//fmt.Printf("dirname is %s\n", dirname)
	return true
}

func ProcessData(procdir string, pmap map[ParamKey][]ParamRef) map[int]map[ParamRef][]Data {
	outdatamap := make(map[int]map[ParamRef][]Data)
	for k := range pmap {
		for _, prefs := range pmap[k] {
			if outdatamap[k.ParrConn] == nil {
				outdatamap[k.ParrConn] = make(map[ParamRef][]Data)
			}
			if CheckDataDir(procdir, k.Host, k.ParrConn) {
				outdatamap[k.ParrConn][prefs] = append(outdatamap[k.ParrConn][prefs], ProcDataFile(procdir, k.Host, k.ParrConn, prefs))
			}
		}

	}
	return outdatamap
}

func WriteCSVHeader(fhand *os.File, pr ParamRef, numelements int) {
	var outln string
	// Alter headers based on parameters passed in
	if pr.Mem == "" && pr.CpuWt == -1 && pr.CpuTm == -1 {
		outln = fmt.Sprintf("Hosts,Connections,Rate,Threads,")
	} else if pr.Mem == "" {
		outln = fmt.Sprintf("Hosts,Connections,Rate,Threads,CPUShares,CPURTPeriod,")
	} else {
		outln = fmt.Sprintf("Hosts,Connections,Rate,Threads,MemoryLimits,CPUShares,CPURTPeriod,")
	}
	for i := 0; i < numelements; i++ {
		outln = outln + "req/s,"
	}
	for i := 0; i < numelements; i++ {
		outln = outln + "kb/s,"
	}
	fhand.WriteString(outln[0:len(outln)-1] + "\n")
}

func WriteCSVData(fhand *os.File, numhosts int, indata map[ParamRef][]Data) {
	var outln string
	frst := bool(false)
	for pr, dataele := range indata {
		if !frst {
			frst = true
			WriteCSVHeader(fhand, pr, len(dataele))
		}
		if pr.Mem == "" && pr.CpuWt == -1 && pr.CpuTm == -1 {
			outln = fmt.Sprintf("%d,%d,%d,%d,", numhosts, pr.Conn, pr.Rate, pr.Threads)
		} else if pr.Mem == "" {
			outln = fmt.Sprintf("%d,%d,%d,%d,%d,%d,", numhosts, pr.Conn, pr.Rate, pr.Threads, pr.CpuWt, pr.CpuTm)
		} else {
			outln = fmt.Sprintf("%d,%d,%d,%d,%s,%d,%d,", numhosts, pr.Conn, pr.Rate, pr.Threads, pr.Mem, pr.CpuWt, pr.CpuTm)
		}
		for _, datum := range dataele {
			outln = outln + fmt.Sprintf("%f,", datum.ReqSec)
		}
		for _, datum := range dataele {
			outln = outln + fmt.Sprintf("%f,", datum.TransSec)
		}
		outln = outln[0:len(outln)-1] + "\n"
		fhand.WriteString(outln)
	}
}

func ExportToCSV(outputfileBase string, indata map[int]map[ParamRef][]Data) {
	fmt.Printf("Exporting to CSV ")
	for h, data := range indata {
		fmt.Printf(".")
		outputfile := fmt.Sprintf("%s_%d.csv", outputfileBase, h)
		file, err := os.Create(outputfile)
		if err != nil {
			fmt.Printf("Encountered error creating file %s. Error was %s\n", outputfile, err.Error())
			return
		}
		WriteCSVData(file, h, data)
		file.Close()
	}
	fmt.Printf("\n")
}

func main() {
	DirToProc := DefaultDataLoc
	fmt.Printf("Processing directories in: %s\n", DirToProc)
	if GetHostList(DirToProc) {
		SrvList, HostList := GetParallelServers(DirToProc)
		parammap := BuildTestParameterMap(DirToProc, SrvList, HostList)
		rawdata := ProcessData(DirToProc, parammap)
		outfile := fmt.Sprintf("%s/dataout", DirToProc)
		ExportToCSV(outfile, rawdata)
	}
}
