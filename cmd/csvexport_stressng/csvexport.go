package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	pts "github.com/sftwrngnr/sampleproject/pkg"
)

var DefaultDataLoc string = "../../perf-testing-data/ec2-huge/run55"
var NoPrompt bool
var GenTimes bool

const DefaultCSVFileName string = "testdata.csv"
const DirectoryFormat string = "%s/%s/"

type ParamKey struct {
	Host     string
	ParrConn int
}

type ParamRef struct {
	Test  string
	Mem   string
	CpuWt float32
	CpuTm int
}

type TimeItems struct {
	hostname  string
	timestart int64
	timeend   int64
}

//type TimeRef struct {
//	TItems []TimeItems
//}

type ByTest []ParamRef

// Sort functions and support
func (t ByTest) Len() int           { return len(t) }
func (t ByTest) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t ByTest) Less(i, j int) bool { return t[i].Test < t[j].Test }

type ByHost []ParamKey

func (t ByHost) Len() int           { return len(t) }
func (t ByHost) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t ByHost) Less(i, j int) bool { return t[i].Host < t[j].Host && t[i].ParrConn < t[j].ParrConn }

type ByConn []ParamKey

func (t ByConn) Len() int           { return len(t) }
func (t ByConn) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t ByConn) Less(i, j int) bool { return t[i].ParrConn < t[j].ParrConn && t[i].Host == t[j].Host }

type Data struct {
	Test    string  //Now handle multiple tests
	BogoOps float64 //BogoOps/s
}

type AggData struct {
	DataBlock []Data
}

var ProcDir []string
var ParmTestList []string

func CheckForCSVFiles(procdir string) bool {
	myDirInfo, err := ioutil.ReadDir(procdir)
	if err != nil {
		log.Printf(fmt.Sprintf("Cannot open directory %s. Failed with error: %s\n", procdir, err.Error()))
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
		if !NoPrompt {
			fmt.Printf("CSV files already exist in %s\nProceed and overwrite? (Y/N)", procdir)
			fmt.Scanf("%s", &ans)
			if strings.EqualFold(ans, "n") {
				return false
			}
		}
	}
	myDirInfo, err := ioutil.ReadDir(procdir)
	if err != nil {
		log.Printf("Cannot open directory %s. Failed with error: %s\n", procdir, err.Error())
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
			log.Printf("Cannot open directory %s. Failed with error: %s\n", myDirInfo, err.Error())
			continue
		}
		for _, file := range myDirInfo {
			if file.IsDir() {
				ifname, err := strconv.Atoi(file.Name())
				if err != nil {
					log.Printf("Error with integer conversion. %s", file.Name())
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
	//fmt.Printf("%v, %v\n", parsrvlist, hostlist)
	return parsrvlist, hostlist
}

func BuildParamsFromFile(fname string) (string, string, float32, int) {
	var cn float32
	var tcn float64
	var ct int
	var testr string
	var m string
	var err error
	memszset := false
	var mytmparr []string
	mystrarr := strings.Split(fname, "_")
	if len(mystrarr) > 2 {
		memszset = true
	}
	for _, spltr := range mystrarr {
		for _, appstr := range strings.Split(spltr, ".") {
			mytmparr = append(mytmparr, appstr)
		}
	}
	sindx := 0
	testr = mytmparr[sindx]
	sindx = sindx + 1
	m = ""
	if memszset {
		m = mytmparr[sindx]
		sindx = sindx + 1
	}
	// We now need to combine the first 2 elements of this array
	mytFloat := mytmparr[sindx] + "." + mytmparr[sindx+1]
	sindx = sindx + 1
	tcn, err = strconv.ParseFloat(mytFloat, 32)
	if err != nil {
		log.Printf("Blew chow on conversion. %s\n", mytmparr[sindx])
	} else {
		cn = float32(tcn)
	}
	sindx = sindx + 1
	ct, err = strconv.Atoi(mytmparr[sindx])
	if err != nil {
		log.Printf("Blew chow on conversion. %s\n", mytmparr[sindx])
	}

	return testr, m, cn, ct
}

func Find(a []string, x string) int {
	for i, n := range a {
		if x == n {
			return i
		}
	}
	return -1
}

func CheckParmRefExists(inlist []ParamRef, addpr ParamRef) bool {
	for _, ckr := range inlist {
		if ckr.Test == addpr.Test && ckr.Mem == addpr.Mem && ckr.CpuWt == addpr.CpuWt && ckr.CpuTm == addpr.CpuTm {
			return true
		}
	}
	return false
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
					//fmt.Printf("Filename is %s\n", file.Name())
					tst, m, cn, ct := BuildParamsFromFile(file.Name())
					myPr := ParamRef{tst, m, cn, ct}
					if !CheckParmRefExists(hdmap[mapkey], myPr) {
						hdmap[mapkey] = append(hdmap[mapkey], myPr)
					}
				}
			}
			//if len(hdmap[mapkey]) > 0 {
			//sort.Sort(ByTest(hdmap[mapkey]))
			//fmt.Printf("%v %v\n", mapkey, hdmap[mapkey])
			//}
		}
	}
	return hdmap
}

func GetTestAndBogoOps(intxt string) Data {
	var retval Data
	mystrs := strings.Split(intxt, " ")
	retval.Test = mystrs[3]
	myval, err := strconv.ParseFloat(strings.Trim(mystrs[8], " "), 64)
	//fmt.Printf("%v\n", mystrs)
	if err != nil {
		log.Printf("Blew chow in BogoOps float conversion. %s\n", err.Error())
	}
	retval.BogoOps = myval
	return retval
}

func GetData(procdir string, hname string, parrconn int, pr ParamRef) []Data {
	var retval []Data
	var fname string
	var ckflg bool
	if pr.Mem == "" {
		fname = fmt.Sprintf("%s/%s/%d/%s.%f_%d.out", procdir, hname, parrconn, pr.Test, pr.CpuWt, pr.CpuTm)
	} else {
		fname = fmt.Sprintf("%s/%s/%d/%s.%s_%f_%d.out", procdir, hname, parrconn, pr.Test, pr.Mem, pr.CpuWt, pr.CpuTm)
	}
	file, err := os.Open(fname)
	if err != err {
		log.Printf("Couldn't read %s\n", fname)
		return retval
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	re_inside_whtsp := regexp.MustCompile(`[\s\p{Zs}]{2,}`)

	for scanner.Scan() {
		txt := scanner.Text()
		if strings.Contains(txt, "stressor") {
			ckflg = true
			continue
		}

		if ckflg && !strings.Contains(txt, "(usr+sys") {
			txt = re_inside_whtsp.ReplaceAllString(txt, " ")
			txt = strings.TrimSpace(txt)
			retval = append(retval, GetTestAndBogoOps(txt))
		}

	}
	return retval
}

func GetTimeData(procdir string, hname string, parrconn int, pr ParamRef) TimeItems {
	var retval TimeItems
	var fname string
	if pr.Mem == "" {
		fname = fmt.Sprintf("%s/%s/%d/%s.%f_%d.out", procdir, hname, parrconn, pr.Test, pr.CpuWt, pr.CpuTm)
	} else {
		fname = fmt.Sprintf("%s/%s/%d/%s.%s_%f_%d.out", procdir, hname, parrconn, pr.Test, pr.Mem, pr.CpuWt, pr.CpuTm)
	}
	begname := fname + ".start"
	endname := fname + ".end"
	bgfile, err := os.Open(begname)
	if err != err {
		log.Printf("Couldn't read %s\n", begname)
		return retval
	}
	defer bgfile.Close()
	endfile, err := os.Open(endname)
	if err != err {
		log.Printf("Couldn't read %s\n", endname)
		return retval
	}
	defer endfile.Close()
	re_inside_whtsp := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
	bgread := io.Reader(bgfile)
	inb := make([]byte, 20)
	nread, berr := bgread.Read(inb)
	if nread == 0 {
		log.Printf("Couldn't read file %s.\n", begname)
		return retval
	}
	txt := string(inb[:])
	txt = re_inside_whtsp.ReplaceAllString(txt, " ")
	txt = strings.TrimSpace(txt)
	begval, berr := strconv.ParseInt(txt, 10, 64)
	if berr != nil {
		log.Printf("Couldn't convert begin time %s %s\n", txt, berr.Error())
	}
	retval.timestart = begval
	retval.hostname = hname
	endread := io.Reader(endfile)
	endb := make([]byte, 20)
	eread, eerr := endread.Read(endb)
	if eread == 0 {
		log.Printf("Couldn't read file %s.\n", endname)
		return retval
	}
	txt = string(endb[:])
	txt = re_inside_whtsp.ReplaceAllString(txt, " ")
	txt = strings.TrimSpace(txt)
	endval, eerr := strconv.ParseInt(txt, 10, 64)
	if eerr != nil {
		log.Printf("Couldn't convert end time %s\n", txt)
	}
	retval.timeend = endval

	return retval
}

func ProcTimeFiles(procdir string, hname string, parrconn int, pref ParamRef) TimeItems {
	var retval TimeItems
	//fmt.Printf("Directory is %s%d/\n", fmt.Sprintf(DirectoryFormat, procdir, hname), parrconn)
	retval = GetTimeData(procdir, hname, parrconn, pref)

	return retval
}

func ProcessTime(procdir string, pmap map[ParamKey][]ParamRef) map[int]map[ParamRef][]TimeItems {
	outtimemap := make(map[int]map[ParamRef][]TimeItems)
	var hlist []string
	var mylist []ParamKey
	var parrcon []int
	for k := range pmap {
		mylist = append(mylist, k)
		myind := sort.SearchStrings(hlist, k.Host)
		if myind < len(hlist) {
			if hlist[myind] == k.Host {
				// Found it
			} else {
				hlist = append(hlist, "")
				copy(hlist[myind+1:], hlist[myind:])
				hlist[myind] = k.Host
			}
		} else {
			hlist = append(hlist, k.Host)
		}
		myind = sort.SearchInts(parrcon, k.ParrConn)
		if myind < len(parrcon) {
			if parrcon[myind] == k.ParrConn {
				continue
			} else {
				parrcon = append(parrcon, 0)
				copy(parrcon[myind+1:], parrcon[myind:])
				parrcon[myind] = k.ParrConn
			}
		} else {
			parrcon = append(parrcon, k.ParrConn)
		}
	}

	for _, tkey := range hlist {
		for _, pcon := range parrcon {
			//fmt.Printf("%s,%d\n", tkey, pcon)
			//for k := range pmap {
			//	fmt.Printf("k is %v\n", k)
			for _, prefs := range pmap[ParamKey{tkey, pcon}] {
				if outtimemap[pcon] == nil {
					outtimemap[pcon] = make(map[ParamRef][]TimeItems)
				}

				outtimemap[pcon][prefs] = append(outtimemap[pcon][prefs], ProcTimeFiles(procdir, tkey, pcon, prefs))
			}
		}
	}
	return outtimemap
}

func WriteTimeData(fhand *os.File, items []TimeItems) {
	for _, ti := range items {
		tdiff := ti.timeend - ti.timestart
		outln := fmt.Sprintf("%s,%d,%d,%d\n", ti.hostname, tdiff, ti.timestart, ti.timeend)
		fhand.WriteString(outln)
	}
}

func ExportTimes(timefile string, inTimeData map[int]map[ParamRef][]TimeItems) {
	for shosts, tdatamap := range inTimeData {
		for pr, items := range tdatamap {
			outputfile := fmt.Sprintf("%s_%d.%s_%f_%d.time", timefile, shosts, pr.Test, pr.CpuWt, pr.CpuTm)
			//fmt.Printf("Time file is %s\n", outputfile)
			// Write to file
			file, err := os.Create(outputfile)
			if err != nil {
				log.Printf("Encountered error creating file %s. Error was %s\n", outputfile, err.Error())
				return
			}
			WriteTimeData(file, items)
			file.Close()
		}
	}
}

func ProcDataFile(procdir string, hname string, parrconn int, pref ParamRef) AggData {
	log.Printf("Directory is %s%d/\n", fmt.Sprintf(DirectoryFormat, procdir, hname), parrconn)
	var retval AggData
	retval.DataBlock = GetData(procdir, hname, parrconn, pref)
	log.Printf("ProcDataFile: %v\n", retval)
	return retval
}

func CheckDataDir(procdir string, host string, parrconn int) bool {
	//dirname := fmt.Sprintf("%s/%s/data/%d", procdir, host, parrconn)
	//fmt.Printf("dirname is %s\n", dirname)
	return true
}

func ProcessData(procdir string, pmap map[ParamKey][]ParamRef) map[int]map[ParamRef][]AggData {
	outdatamap := make(map[int]map[ParamRef][]AggData)
	for k := range pmap {
		//fmt.Printf("k is %v\n", k)
		for _, prefs := range pmap[k] {
			if outdatamap[k.ParrConn] == nil {
				outdatamap[k.ParrConn] = make(map[ParamRef][]AggData)
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
	if pr.Mem == "" && pr.CpuWt == -1.0 && pr.CpuTm == -1 {
		outln = fmt.Sprintf("Hosts,Test,")
	} else if pr.Mem == "" {
		outln = fmt.Sprintf("Hosts,Test,CPUShares,CPU Allocation,")
	} else {
		outln = fmt.Sprintf("Hosts,Test,MemoryLimits,CPUShares,CPU Allocation,")
	}
	for i := 0; i < numelements; i++ {
		outln = outln + "bogo ops/s,"
	}
	fhand.WriteString(outln[0:len(outln)-1] + "\n")
}

func SAContains(needle string, haystack []string) bool {
	for _, itm := range haystack {
		if needle == itm {
			return true
		}
	}
	return false
}

func AddTestSets(insl []ParamRef, indata map[ParamRef][]AggData) map[ParamRef][]string {
	var retval map[ParamRef][]string = make(map[ParamRef][]string)
	for _, de := range insl {
		var tests []string
		for _, datum := range indata[de] {
			for _, tdata := range datum.DataBlock {
				if !SAContains(tdata.Test, tests) {
					tests = append(tests, tdata.Test)
				}
			}
		}
		retval[de] = tests
	}
	return retval
}

func GenOutputFileName(dp string, tstnms []string, pref ParamRef, nh int) string {
	retval := dp + "/"
	for _, tst := range tstnms {
		retval = retval + tst + "_"
	}
	if pref.Mem != "" {
		retval = retval + pref.Mem + "_"
	}
	if pref.CpuWt != 0.0 {
		retval = retval + fmt.Sprintf("%f_", pref.CpuWt)
	}
	if pref.CpuTm != 0 {
		retval = retval + fmt.Sprintf("%d_", pref.CpuTm)
	}
	retval = retval + fmt.Sprintf("%d.csv", nh)
	log.Printf("%s\n", retval)
	return retval
}

func WriteCSVData(dirpath string, numhosts int, indata map[ParamRef][]AggData) {
	var outln string
	// The indata map is by definition, unsorted. Build the stringlist, so we
	// can have a consistent order
	var nslice []ParamRef
	for k, _ := range indata {
		nslice = append(nslice, k)
	}
	sort.Sort(ByTest(nslice))
	tsdata := AddTestSets(nslice, indata)
	log.Printf("%v\n", tsdata)
	for _, ns := range nslice {
		outputfile := GenOutputFileName(dirpath, tsdata[ns], ns, numhosts)
		fhand, err := os.Create(outputfile)
		if err != nil {
			log.Printf("Encountered error creating file %s. Error was %s\n", outputfile, err.Error())
			return
		}
		dataele := indata[ns]
		WriteCSVHeader(fhand, ns, len(dataele))
		// Figure out how many tests per data set
		numtests := len(tsdata[ns])
		for ol := 0; ol < numtests; ol++ {
			var begln bool
			var TestName string
			for _, datum := range dataele {
				if !begln {
					begln = true
					TestName = tsdata[ns][ol]
					if ns.Mem == "" && ns.CpuWt == -1.0 && ns.CpuTm == -1 {
						outln = fmt.Sprintf("%d,%s,", numhosts, TestName)
					} else if ns.Mem == "" {
						outln = fmt.Sprintf("%d,%s,%f,%d,", numhosts, TestName, ns.CpuWt, ns.CpuTm)
					} else {
						outln = fmt.Sprintf("%d,%s,%s,%f,%d,", numhosts, TestName, ns.Mem, ns.CpuWt, ns.CpuTm)
					}
				}
				log.Printf("%v\n", datum)
				for _, subdata := range datum.DataBlock {
					if subdata.Test == TestName {
						outln = outln + fmt.Sprintf("%f,", subdata.BogoOps)
					}
				}
			}
			outln = outln[0:len(outln)-1] + "\n"
			fhand.WriteString(outln)
			outln = ""
		}
		fhand.Close()
	}
}

func ExportToCSV(outputfileBase string, indata map[int]map[ParamRef][]AggData) {
	log.Printf("Exporting to CSV ")
	for h, data := range indata {
		//fmt.Printf(".")
		//outputfile := fmt.Sprintf("%s_%d.csv", outputfileBase, h)
		WriteCSVData(outputfileBase, h, data)
	}
	//fmt.Printf("\n")
}

func main() {
	pts.PTLogger().Info("csvexport_stressng started.")
	defer pts.PTLogger().CloseLog()

	PathPtr := flag.String("path", DefaultDataLoc, "Path string")
	NPPtr := flag.Bool("n", false, "No prompt mode.")
	TPtr := flag.Bool("t", false, "Generate time summaries.")

	flag.Parse()

	log.Println("path", *PathPtr)
	log.Println("No prompt mode:", *NPPtr)
	log.Println("Generate time summaries:", *TPtr)

	NoPrompt = *NPPtr
	GenTimes = *TPtr
	DirToProc := *PathPtr
	log.Printf("Processing directories in: %s\n", DirToProc)
	if GetHostList(DirToProc) {
		SrvList, HostList := GetParallelServers(DirToProc)
		parammap := BuildTestParameterMap(DirToProc, SrvList, HostList)
		rawdata := ProcessData(DirToProc, parammap)
		ExportToCSV(DirToProc, rawdata)
		if GenTimes {
			rawtime := ProcessTime(DirToProc, parammap)
			timefile := fmt.Sprintf("%s/runtimes", DirToProc)
			ExportTimes(timefile, rawtime)
		}
	}
}
