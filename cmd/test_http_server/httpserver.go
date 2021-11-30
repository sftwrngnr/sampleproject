package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/copier"
	pts "github.com/sftwrngnr/sample_project/pkg"
	twsref "github.com/sftwrngnr/sampleproject/TestWebServer"
)

var sigDone = make(chan bool, 1)

const TestSessionName = "CS_TestFramework"
const SessTimeout = 432000 // Expires after 5 days

var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

type SelectData struct {
	Value string
	Label string
}

type ReplaceData struct {
	SearchStr       string
	ReplaceStrSlice []string
	ReplaceStr      string
}

type TestWebServer struct {
	IPAddy        string
	Port          int
	Daemon        bool
	WebServerRoot string
	SessionRef    *twsref.TWSSession
	ValidTests    []SelectData
	ValidPermute  []SelectData
	ValidHosts    []SelectData
	CSOptions     []SelectData
	TRDir         string
	TRFile        string
	FTPFile       string
	CurrFormData  url.Values
}

func NewWebServer(yamlref *twsref.TestWSYaml) *TestWebServer {

	srvAddy, srvPort := yamlref.GetServerParams()
	return &TestWebServer{
		IPAddy:        srvAddy,
		Port:          srvPort,
		Daemon:        yamlref.DaemonProc,
		WebServerRoot: yamlref.WebServerRoot,
		TRDir:         yamlref.TestRunnerDir,
		TRFile:        yamlref.TestRunnerOutput,
		FTPFile:       fmt.Sprintf("%s/ftpclient.yaml", yamlref.FTPServerDir),
		SessionRef:    twsref.NewSession(yamlref.SessionDir),
	}
}

func (tws *TestWebServer) WaitForKeyPress() {
	if !tws.Daemon {
		fmt.Printf("Press return to exit the server.\n")
		reader := bufio.NewReader(os.Stdin)
		reader.ReadString('\n')
		sigDone <- true
	}
}

func (tws *TestWebServer) Run() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, os.Kill)
	go func() {
		// Select on signals
		for {
			select {
			case <-sigCh:
				if !tws.Daemon {
					log.Printf("Interrupt!\n")
				}
				sigDone <- true
				break
			}
		}

	}()

	go tws.WaitForKeyPress()

	<-sigDone
}

func (tws *TestWebServer) AddOptions(indata []SelectData) string {
	var retval string
	for _, vals := range indata {
		retval = retval + fmt.Sprintf("<option value=\"%s\">%s</option>\r", vals.Value, vals.Label)
	}
	return retval
}

func (tws *TestWebServer) AddVarBlock() string {
	var retval string
	retval = "<script>\n"
	for idx, vals := range tws.CurrFormData {
		log.Printf("%s:%v\n", idx, vals)
		if idx == "target" {
			retval = retval + fmt.Sprintf("var Target=\"%s\"\n", vals[0])
		}
		if idx == "testseq" {
			retval = retval + fmt.Sprintf("var TestSeq=\"%s\"\n", vals[0])
		}
		if idx == "test" {
			retval = retval + fmt.Sprintf("var Test=\"%s\"\n", vals[0])
			numTests := strings.Count(vals[0], ",") + 1
			if numTests <= 0 {
				numTests = 1
			}
			retval = retval + fmt.Sprintf("var NumTests=\"%d\"\n", numTests)
		}
		if idx == "permute_method" {
			permqty := ""
			switch vals[0] {
			case "0":
				retval = retval + fmt.Sprintf("var PermuteMethod=\"%s\"\n", "Execute Individually")
			case "1":
				retval = retval + fmt.Sprintf("var PermuteMethod=\"%s\"\n", "Permute All")
				permqty = tws.CurrFormData["permuteqty"][0]
			case "2":
				retval = retval + fmt.Sprintf("var PermuteMethod=\"%s\"\n", "Execute Simultaneously")
			}
			retval = retval + fmt.Sprintf("var PermuteQty=\"%s\"\n", permqty)
		}
	}
	retval = retval + "</script>\n"
	return retval
}

func (tws *TestWebServer) RemoveString(instr string, remstr string) string {
	fnd := strings.Index(instr, remstr)
	if fnd != -1 {
		return instr[0:fnd] + instr[(fnd+len(remstr)):]
	} else {
		return instr
	}
}

func (tws *TestWebServer) AddJavaVariables(invarnm string, vals []SelectData) string {
	var outstr string
	for i, val := range vals {
		if i == 0 {
			outstr = fmt.Sprintf("var %s_val = \"%s\";\n", invarnm, val.Value)
			outstr = outstr + fmt.Sprintf("var %s_lab = \"%s\";\n", invarnm, val.Label)
		} else {
			outstr = outstr + fmt.Sprintf("%s_val = %s_val + \",%s\";\n", invarnm, invarnm, val.Value)
			outstr = outstr + fmt.Sprintf("%s_lab = %s_lab + \",%s\";\n", invarnm, invarnm, val.Label)
		}
	}
	return outstr
}

func (tws *TestWebServer) AddServerRef(refsrver string) string {
	var outstr string = fmt.Sprintf("\tvar statsrv = \"http://%s\";\n", fmt.Sprintf("%s/status", refsrver))
	outstr = outstr + fmt.Sprintf("\tvar statetran = \"http://%s\";\n", fmt.Sprintf("%s/transition", refsrver))
	return outstr
}

func (tws *TestWebServer) ProcessFillData(Indata []string, sessId string, refsrver string) string {
	var retstr = ""
	for _, tdata := range Indata {
		retstr = retstr + tdata
		if strings.Contains(tdata, "ServerRef") {
			retstr = tws.RemoveString(retstr, "<!ServerRef>")
			retstr = retstr + tws.AddServerRef(refsrver)
		}
		if strings.Contains(tdata, "TEST_ENTRY") {
			retstr = tws.RemoveString(retstr, "<!TEST_ENTRY>")
			retstr = retstr + tws.AddJavaVariables("test_entry_opts", tws.ValidTests)
			//retstr = retstr + tws.AddOptions(tws.ValidTests)
		}
		if strings.Contains(tdata, "PERMUTE_ENTRY") {
			retstr = tws.RemoveString(retstr, "<!PERMUTE_ENTRY>")
			retstr = retstr + tws.AddJavaVariables("permute_entry_opts", tws.ValidPermute)
			//retstr = retstr + tws.AddOptions(tws.ValidPermute)
		}
		if strings.Contains(tdata, "TARGET_ENTRY") {
			retstr = retstr + tws.AddOptions(tws.ValidHosts)
			//retstr = tws.RemoveString(retstr, "<!TARGET_ENTRY>")
			//retstr = retstr + tws.AddJavaVariables("test_targets", tws.ValidHosts)
		}
		if strings.Contains(tdata, "CS_OPTIONS") {
			retstr = tws.RemoveString(retstr, "<!CS_OPTIONS>")
			retstr = retstr + tws.AddJavaVariables("com_str_opts", tws.CSOptions)
		}
		if strings.Contains(tdata, "<!VAR_BLOCK>") {
			retstr = retstr + tws.AddVarBlock()
		}
		if strings.Contains(tdata, "SessionId") {
			retstr = tws.RemoveString(retstr, "<!SessionId>")
			retstr = retstr + fmt.Sprintf("var sessionId =\"%s\";", sessId)
		}
	}
	return retstr
}

func ReadFileLines(fname string) []string {
	fhand, ferr := os.Open(fname)
	if ferr != nil {
		panic(ferr)
	}
	defer fhand.Close()
	var retval []string
	scanner := bufio.NewScanner(fhand)
	for scanner.Scan() {
		retval = append(retval, scanner.Text())
	}
	return retval
}

func (tws *TestWebServer) GenYamlFile(inVals url.Values) []string {
	var retval []string
	log.Printf("inVals is: %v\n", inVals)
	var myTd twsref.TestDef
	for key, _ := range inVals {
		if key == "SessionID" {
			continue
		}
		fmt.Printf("key:%v inVals[key]:%v \n", key, inVals[key])
		derr := json.Unmarshal([]byte(inVals[key][0]), &myTd)
		if derr != nil {
			log.Printf("Error unmarshalling json data in GenYamlFile. %s\n", derr.Error())
			retval[0] = "Catastrophic system failure"
			return retval
		}
	}
	templatefile := fmt.Sprintf("./templates/%s.yaml", strings.Replace(myTd.TestInstance, ".", "_", 3))
	retval = ReadFileLines(templatefile)
	// Build test_types entry
	var outstr string = "   test_types : ["
	var addstr string

	for _, instr := range inVals["test"] {
		for _, tst := range strings.Split(instr, ",") {
			if addstr != "" {
				addstr = addstr + ", "
			}
			addstr = addstr + fmt.Sprintf("\"%s\"", tst)
		}
	}
	outstr = outstr + addstr + "]"
	retval = append(retval, outstr)
	retval = append(retval, fmt.Sprintf("   duration : %s", inVals["duration"][0]))
	csfx := fmt.Sprintf("   command_suffix : ")

	if inVals["csoptions"] != nil {
		csfx = csfx + inVals["csoptions"][0]
	}

	if inVals["cmdsfx"] != nil {
		csfx = csfx + inVals["cmdsfx"][0]
	}
	csfx = csfx + " --metrics"
	retval = append(retval, csfx)
	retval = append(retval, "   permute_settings:")
	pset, err := strconv.Atoi(inVals["permute_method"][0])
	if err != nil {
		panic(err)
	}
	outstr = "       permute_type: "
	switch pset {
	case 0:
		outstr = outstr + "single_run_each"
	case 1:
		outstr = outstr + "permute_all"
	case 2:
		outstr = outstr + "all_at_once"
	}
	retval = append(retval, outstr)
	if pset == 1 {
		//outstr = "       permute_qty: "
		pqty, pqerr := strconv.Atoi(inVals["permuteqty"][0])
		if pqerr != nil {
			panic(err)
		}
		retval = append(retval, fmt.Sprintf("       permute_qty: %d", pqty))
	}
	retval = append(retval, fmt.Sprintf("   containers : %s", inVals["testseq"]))
	if (inVals["DockMemLimits"] != nil) || (inVals["DockCPUShares"] != nil) || (inVals["DockCPURTPeriod"] != nil) {
		retval = append(retval, "   ContainerSettings: ")
		if inVals["DockMemLimits"] != nil {
			pstr := inVals["DockMemLimits"][0]
			log.Printf("%v\n", pstr)
			if strings.Count(pstr, ",") > 0 {
				outstr = ""
				for _, tstr := range strings.Split(pstr, ",") {
					outstr = outstr + fmt.Sprintf("\"%s\",", tstr)
				}
				outstr = outstr[:len(outstr)-1]
			} else {
				outstr = fmt.Sprintf("\"%s\"", pstr)
			}
			retval = append(retval, fmt.Sprintf("        MemoryLimits : [%s]", outstr))
		}
		retval = append(retval, "        CPULimits:")
		if inVals["DockCPUShares"] != nil {
			retval = append(retval, fmt.Sprintf("            CPUShares: [%s]", inVals["DockCPUShares"][0]))
		}
		if inVals["DockCPURTPeriod"] != nil {
			retval = append(retval, fmt.Sprintf("            CPURTPeriod: [%s]", inVals["DockCPURTPeriod"][0]))
		}
	}
	return retval
}

func (tws *TestWebServer) ServeCss(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	log.Printf("%v\n", r.URL)
}

func (tws *TestWebServer) errorHandler(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)
	if status == http.StatusNotFound {
		fmt.Fprint(w, "404 Page Not Found")
	}
}

func (tws *TestWebServer) ServeRoot(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	myUUID := uuid.New()
	currSess := fmt.Sprintf("%s", myUUID)
	log.Printf("New UUID is %s\n", myUUID)
	if (r.URL.Path != "/") && (r.URL.Path != "/index.html") {
		tws.errorHandler(w, r, http.StatusNotFound)
		return
	}
	if tws.WebServerRoot != "" {
		session, err := store.Get(r, TestSessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Check to see if this session is active or not. If it isn't, replace
		// with new session.
		val := session.Values["TestId"]
		if val != nil && tws.SessionRef.CheckValidSession(val.(string)) {
			log.Printf("Found session %s still active.\n", val.(string))
			currSess = val.(string)
			var tData twsref.TWSSessionData
			tws.SessionRef.GetSessionData(val.(string), tData)
			// Update last accessed
			tws.SessionRef.AddSessionData(val.(string), tData)
			// Check Status
		} else {
			// Set some session values.
			if val != nil {
				if tws.SessionRef.CheckValidSession(val.(string)) {
					// Let's delete the session and session reference
					session.Options.MaxAge = -1
					session.Save(r, w)
				}
			}
			session.Options.MaxAge = SessTimeout //Explicitly set to timeout
			session.Values["TestId"] = currSess

			// Save it before we write to the response/return from the handler.
			err = session.Save(r, w)
			if err == nil {
				if !tws.SessionRef.CreateNewSession(currSess) {
					http.Error(w, "Error writing to server session store.", http.StatusInternalServerError)
				} else {
					var twsData twsref.TWSSessionData
					twsData.CurrentState = twsref.UnInit
					if !tws.SessionRef.AddSessionData(currSess, twsData) {
						http.Error(w, "Error with AddSessionData", http.StatusInternalServerError)
						return
					}
				}

			}
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if !strings.Contains(r.URL.Path, ".css") {
			// Read Index and output
			FileName := fmt.Sprintf("%s/index.html", tws.WebServerRoot)
			outstr := tws.ProcessFillData(ReadFileLines(FileName), currSess, fmt.Sprintf("%v:%v", tws.IPAddy, tws.Port))
			fmt.Fprintf(w, outstr)
		} else {
			myFile, err := os.Open(fmt.Sprintf("%s%s", tws.WebServerRoot, r.URL.Path))
			if err != nil {
				log.Printf("Blew chow on os.Open %s\n", err)
				return
			}
			// First get session cookie, get status, and then determine what to
			// do.
			http.ServeContent(w, r, r.URL.Path, time.Time{}, myFile)

		}

	} else {
		fmt.Fprintf(w, "<H1>Performance test configuration</H1>") // send data to client side
		mysrvStr := fmt.Sprintf("%v:%v", tws.IPAddy, tws.Port)
		fmt.Fprintf(w, "Running on %s NO WebServerRoot (webserverroot) specified.", mysrvStr)
	}
}

func (tws *TestWebServer) WriteOutputYaml(w http.ResponseWriter, instr []string) {
	outfilename := fmt.Sprintf("%s/%s", tws.TRDir, tws.TRFile)
	f, err := os.Create(outfilename)
	if err != nil {
		fmt.Fprintf(w, "Error could not write to yaml file %s. Test cannot be run", outfilename)
		return
	}
	defer f.Close()
	for _, s := range instr {
		f.WriteString(fmt.Sprintf("%s\n", s))
	}
}

func (tws *TestWebServer) TerminateTest(w http.ResponseWriter, r *http.Request) {
	// Write yaml file
	var req pts.PTRpcSrvReq
	retstat := new(pts.ReturnStatus)
	r.ParseForm()
	tws.CurrFormData = r.Form
	req.ReqCmd = pts.TerminateTest
	err := pts.RpcClientRequest(&req, "127.0.0.1", retstat)
	if err != nil {
		log.Printf("%s", err.Error())
	} else {
		log.Printf("Success: %v", retstat)
	}
	r.URL.Path = "/"
	tws.ServeRoot(w, r)
}

func (tws *TestWebServer) UpdateSessionFile(uuid string, myTd twsref.TestDef, myNewState twsref.SessState, noStateTran bool) {
	var twsData twsref.TWSSessionData
	if tws.SessionRef.GetSessionData(uuid, twsData) {
		// We have an active session
		copier.Copy(&twsData.TestDefinition, &myTd)
		if !noStateTran {
			twsData.CurrentState = myNewState
		}
		tws.SessionRef.AddSessionData(uuid, twsData)
	}
}

func (tws *TestWebServer) GenerateTestDefFile(sessId string, myTd twsref.TestDef) bool {
	retval := false

	return retval
}

func (tws *TestWebServer) FTPTransferTestDefFile(sessId string, myTd twsref.TestDef) bool {
	var req pts.PTRpcSrvReq
	retstat := new(pts.ReturnStatus)
	retval := false
	req.ReqCmd = pts.ExecuteTest
	//err := pts.RpcClientRequest(&req, "127.0.0.1", retstat)
	pts.RpcClientRequest(&req, "127.0.0.1", retstat)

	return retval
}

func (tws *TestWebServer) ExecuteTest(w http.ResponseWriter, r *http.Request) {
	// Write yaml file
	//var req pts.PTRpcSrvReq
	//retstat := new(pts.ReturnStatus)
	r.ParseForm()
	tws.CurrFormData = r.Form
	// We need to unmarshal the objects passed into us
	var myTd twsref.TestDef
	sessId := r.Form["SessionID"][0]
	derr := json.Unmarshal([]byte(r.Form["TestDef"][0]), &myTd)
	if derr != nil {
		log.Printf("Error unmarshalling json data in ExecuteTest. %s\n", derr.Error())
	}

	tws.UpdateSessionFile(sessId, myTd, twsref.TestDefined, false)
	//Now, rather than generate a YAML file that contains the test definition,
	//we're instead going to generate a testdef file in /tmp, ftp that to the
	//target machine, and then execute the test. Once the test has been
	//completed, we can then update the state and proceed from there.
	if tws.GenerateTestDefFile(sessId, myTd) {
		tws.UpdateSessionFile(sessId, myTd, twsref.TestExecFileWritten, false)
		if tws.FTPTransferTestDefFile(sessId, myTd) {
			tws.UpdateSessionFile(sessId, myTd, twsref.TestDefTransferSuccess, false)
			// We can now launch the test.

		} else {
			tws.UpdateSessionFile(sessId, myTd, twsref.TestDefTransferFailed, false)
		}

	} else {
		tws.UpdateSessionFile(sessId, myTd, twsref.TestExecFileWriteFail, false)
	}

	/*
		outstr := tws.GenYamlFile(r.Form)
		tws.WriteOutputYaml(w, outstr)
		// Now read generated YAML file and write equivalent dat to FTP file
		sngyaml := fmt.Sprintf("%s/%s", tws.TRDir, tws.TRFile)
		// Read this file using existing reader
		var stressng pts.StressNGTest
		if pts.LoadStressNGTestYamlFile(sngyaml, &stressng) {
			// Use this to read and write the ftp file
			var ftpcli pts.FTPClientConfig
			if pts.LoadFTPConfigYamlFile(tws.FTPFile, &ftpcli) {
				// Update data and write to yaml file
				ftpcli.Host = stressng.ServerHostCfg.Host
				ftpcli.User = stressng.ServerHostCfg.User
				ftpcli.Password = stressng.ServerHostCfg.Password
				ftpcli.Sshcert = stressng.ServerHostCfg.Sshcert
				ftpcli.SSHCertPass = stressng.ServerHostCfg.SSHCertPass
				ftpcli.Sshprivatekey = stressng.ServerHostCfg.Sshprivatekey
				ftpcli.SSHPubKey = stressng.ServerHostCfg.SSHPubKey
				ftpcli.RetrieveDataLoc = stressng.ServerHostCfg.Datadir
				pts.WriteFTPConfigYamlFile(tws.FTPFile, &ftpcli)
			}

		}

		req.ReqCmd = pts.ExecuteTest
		err := pts.RpcClientRequest(&req, "127.0.0.1", retstat)
		if err != nil {
			log.Printf("%s", err.Error())
		} else {
			log.Printf("Success: %v", retstat)
			FileName := fmt.Sprintf("%s/testrun.html", tws.WebServerRoot)
			//data, err := ioutil.ReadFile(FileName)

			outstr := tws.ProcessFillData(ReadFileLines(FileName), "", fmt.Sprintf("%v:%v", tws.IPAddy, tws.Port))
			fmt.Fprintf(w, outstr)
		}
	*/
	/*
		fmt.Fprintf(w, "<H1>Test request submitted</H1>")
		fmt.Fprintf(w, "Values, %s", r.Form)
		outstr := tws.GenYamlFile(r.Form)
		fmt.Fprintf(w, "Generated yaml file: \n")
		fmt.Fprintf(w, "<pre>")
		for _, t := range outstr {
			fmt.Fprintf(w, "%s\n", t)
		}
		fmt.Fprintf(w, "</pre>")
	*/
}

func (tws *TestWebServer) BuildPipedString(instr []string) string {
	var retval string
	log.Printf("%s\n", instr)
	for _, mystr := range instr {
		retval = retval + mystr + "|"
	}
	return retval
}

func (tws *TestWebServer) ProcFTPVars(inVals url.Values) []string {
	var retval []string
	log.Printf("ProcFTPVars %v\n", inVals)
	retval = append(retval, "<script>")
	retval = append(retval, fmt.Sprintf("var TestParms=\"%s\"", inVals["TestParams"]))
	retval = append(retval, fmt.Sprintf("var Test=\"%s\"", inVals["Tests"][0]))
	retval = append(retval, fmt.Sprintf("var NumTests=\"%s\"", inVals["NTests"][0]))
	retval = append(retval, fmt.Sprintf("var Target=\"%s\"", inVals["TargetSrv"][0]))
	retval = append(retval, fmt.Sprintf("var TestSeq=\"%s\"", inVals["TSeq"][0]))
	retval = append(retval, fmt.Sprintf("var PermuteMethod=\"%s\"", inVals["PermMethod"][0]))
	retval = append(retval, "</script>")
	return retval
}

func (tws *TestWebServer) ProcessSubstitution(Indata []string, findstr string, replacestr []string) string {
	var retstr = ""
	for _, tdata := range Indata {
		if strings.Contains(tdata, findstr) {
			for _, mystr := range replacestr {
				retstr = retstr + mystr + "\n"
			}
		} else {
			retstr = retstr + tdata
		}
	}
	return retstr
}

func (tws *TestWebServer) ProcessMultipleSubstitutions(Indata []string, repData []ReplaceData) string {
	log.Printf("ProcessMuldipleSubstitutions(repData %v", repData)
	var retstr = ""
	for _, tdata := range Indata {
		var fnd bool
		for _, findval := range repData {
			if strings.Contains(tdata, findval.SearchStr) {
				// Append portion of tdata up to SearchStr to retstr
				fnd = true
				myLoc := strings.Index(tdata, findval.SearchStr)
				retstr = retstr + tdata[0:myLoc]
				if findval.ReplaceStr != "" {
					retstr = retstr + findval.ReplaceStr
				} else {
					for _, mystr := range findval.ReplaceStrSlice {
						retstr = retstr + mystr + "\n"
					}
				}
				log.Printf("len(tdata) is %d, endVal is %d\n", len(tdata), (myLoc + len(findval.SearchStr)))
				if len(tdata) > (myLoc + len(findval.SearchStr)) {
					retstr = retstr + tdata[myLoc+len(findval.SearchStr):] + "\n"
				}
			}
		}
		if !fnd {
			retstr = retstr + tdata
		}
	}
	return retstr
}

func (tws *TestWebServer) ExecuteSFTP(w http.ResponseWriter, r *http.Request) {
	// Write yaml file
	var req pts.PTRpcSrvReq
	retstat := new(pts.ReturnStatus)
	r.ParseForm()
	PlaceVars := tws.ProcFTPVars(r.Form)
	log.Printf("%v\n", PlaceVars)
	req.ReqCmd = pts.SFTPDownload
	err := pts.RpcClientRequest(&req, "127.0.0.1", retstat)
	if err != nil {
		log.Printf("%s", err.Error())
	} else {
		log.Printf("Success: %v", retstat)
		FileName := fmt.Sprintf("%s/ftpretrieve.html", tws.WebServerRoot)
		//data, err := ioutil.ReadFile(FileName)

		outstr := tws.ProcessSubstitution(ReadFileLines(FileName), "<!CS_VARS_GO_HERE>", PlaceVars)
		fmt.Fprintf(w, outstr)
	}
}

func (tws *TestWebServer) UpdateReadme(w http.ResponseWriter, r *http.Request) {
	var rData []ReplaceData
	var tData []ReplaceData
	r.ParseForm()
	FileName := fmt.Sprintf("%s/updatereadme.html", tws.WebServerRoot)
	//PlaceVars := tws.ProcFTPVars(r.Form)
	mFileLines := ReadFileLines("templates/README")
	log.Printf("Before Processing: %v\n", mFileLines)
	tData = append(tData, ReplaceData{SearchStr: "<INSTANCE_NAME>", ReplaceStr: r.Form["TargetSrv"][0]})
	tData = append(tData, ReplaceData{SearchStr: "<TEST_PERMUTE_TYPE>", ReplaceStr: r.Form["PermMethod"][0]})
	tData = append(tData, ReplaceData{SearchStr: "<NUM_TESTS_EXECUTED>", ReplaceStr: r.Form["NTests"][0]})
	tData = append(tData, ReplaceData{SearchStr: "<TESTS_EXECUTED>", ReplaceStr: r.Form["Tests"][0]})
	tData = append(tData, ReplaceData{SearchStr: "<TEST_SEQ>", ReplaceStr: r.Form["TSeq"][0]})
	//t :=  time.Now()
	tData = append(tData, ReplaceData{SearchStr: "<RUN_DATE>", ReplaceStr: fmt.Sprintf("%s", time.Now().Format("Mon Jan _2 15:04:05 2006"))})
	mTmpLines := tws.ProcessMultipleSubstitutions(mFileLines, tData)
	log.Printf("Currently %v\n", mTmpLines)
	tmpstr := tws.ProcessSubstitution(strings.Split(mTmpLines, "\n"), "<TESTRUNSETTINGS>", r.Form["TestParams"])
	tmpstr = strings.Replace(tmpstr, "|", "\n", -1)
	rData = append(rData, ReplaceData{SearchStr: "<!CS_VARS_GO_HERE>", ReplaceStr: fmt.Sprintf("<script>\nvar DPath=\"%s\"\n </script>\n", r.Form["DPath"][0])})
	rData = append(rData, ReplaceData{SearchStr: "<!CS_README_CONTENT_GOES_HERE>", ReplaceStr: tmpstr})
	outstr := tws.ProcessMultipleSubstitutions(ReadFileLines(FileName), rData)

	fmt.Fprintf(w, outstr)
}

func (tws *TestWebServer) GithubCommit(w http.ResponseWriter, r *http.Request) {
	// Write yaml file
	r.ParseForm()
	FileName := fmt.Sprintf("%s/githubcommit.html", tws.WebServerRoot)
	outstr := tws.ProcessFillData(ReadFileLines(FileName), "", fmt.Sprintf("%v:%v", tws.IPAddy, tws.Port))
	fmt.Fprintf(w, outstr)
}

func (tws *TestWebServer) OutStr(mystr string) {
	if !tws.Daemon {
		log.Printf("%s", mystr)
	}
}

func (tws *TestWebServer) RetrieveStatus(w http.ResponseWriter, r *http.Request) {
	// This will return status info for use with the test page.
	r.ParseForm()
	var req pts.PTRpcSrvReq
	retstat := new(pts.ReturnStatus)
	req.ReqCmd = pts.GetStatus
	retstat = new(pts.ReturnStatus)
	pts.RpcClientRequest(&req, "127.0.0.1", retstat)
	outstr, err := json.MarshalIndent(retstat, "", "    ")
	if err != nil {
		fmt.Fprintf(w, "Error marshalling status data.")
	} else {
		//enc := json.NewEncoder(w)
		//fmt.Fprintf(w, "<div stat-data='")
		//enc.Encode(retstat)
		fmt.Fprintf(w, string(outstr))
		//fmt.Fprintf(w, "'></div>")
	}

}

func (tws *TestWebServer) WriteReadme(readme []string, PathStr string) {
	log.Printf("WriteReadme %s", PathStr)
	f, err := os.Create(fmt.Sprintf("%s/README", PathStr))
	if err != nil {
		log.Printf("Error could not write to %s/README file.", PathStr)
		return
	}
	defer f.Close()
	for _, s := range readme {
		f.WriteString(fmt.Sprintf("%s\n", s))
	}

}

func (tws *TestWebServer) CsvAndCommit(w http.ResponseWriter, r *http.Request) {
	// This will return status info for use with the test page.
	log.Printf("CsvAndCommit")
	r.ParseForm()
	// Get Readme and path info.
	ReadmeInfo := r.Form["ReadmeData"]
	PathStr := r.Form["DataPath"]
	tws.WriteReadme(ReadmeInfo, PathStr[0])
	// Serve page and launch csv and commit process
	FileName := fmt.Sprintf("%s/csvandcommit.html", tws.WebServerRoot)
	outstr := ReadFileLines(FileName)
	fmt.Fprintf(w, strings.Join(outstr, "\n"))

	var req pts.PTRpcSrvReq
	retstat := new(pts.ReturnStatus)
	req.ReqCmd = pts.CSVAndCommit
	req.StatusMsg = append(req.StatusMsg, PathStr[0])
	err := pts.RpcClientRequest(&req, "127.0.0.1", retstat)
	if err != nil {
		log.Printf("%s", err.Error())
	} else {
		log.Printf("Success: %v", retstat)
	}
}

func (tws *TestWebServer) SetupWebServer() {
	http.HandleFunc("/", tws.ServeRoot)
	http.HandleFunc("/css", tws.ServeCss)
	http.HandleFunc("/execute_test", tws.ExecuteTest)
	http.HandleFunc("/terminate_test", tws.TerminateTest)
	http.HandleFunc("/sftp_getdata", tws.ExecuteSFTP)
	http.HandleFunc("/UpdateReadme", tws.UpdateReadme)
	http.HandleFunc("/CsvAndCommit", tws.CsvAndCommit)
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir(fmt.Sprintf("%s/js", tws.WebServerRoot)))))
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(fmt.Sprintf("%s/images", tws.WebServerRoot)))))
	http.HandleFunc("/status", tws.RetrieveStatus)
	//http.HandleFunc(ws.TestRequest, tws.HandleTestRequest)
	mysrvStr := fmt.Sprintf("%v:%v", tws.IPAddy, tws.Port)
	tws.OutStr(fmt.Sprintf("Webserver running on %v\n", mysrvStr))
	go http.ListenAndServe(mysrvStr, nil)
}

func ProcessCmdLine() *string {

	fname := flag.String("config", "./profilews.yaml", "Location for config file.")
	if flag.NFlag() == 0 {
		log.Printf("No flags specified, using defaults. (./profilews.yaml)\n")
		flag.PrintDefaults()
	}
	return fname
}

func FillHCData(wcData *TestWebServer) {
	wcData.ValidTests = append(wcData.ValidTests, SelectData{"cpu", "CPU"})
	wcData.ValidTests = append(wcData.ValidTests, SelectData{"filesystem", "Filesystem"})
	wcData.ValidTests = append(wcData.ValidTests, SelectData{"io", "I/O"})
	wcData.ValidTests = append(wcData.ValidTests, SelectData{"memory", "Memory"})
	wcData.ValidTests = append(wcData.ValidTests, SelectData{"msg", "Message"})
	wcData.ValidTests = append(wcData.ValidTests, SelectData{"pipe", "Pipe"})
	wcData.ValidTests = append(wcData.ValidTests, SelectData{"copyfile", "Copy File"})
	wcData.ValidTests = append(wcData.ValidTests, SelectData{"hdd", "Hard Disk Drive"})
	wcData.ValidTests = append(wcData.ValidTests, SelectData{"udp", "UDP"})
	wcData.ValidPermute = append(wcData.ValidPermute, SelectData{"0", "Run each test individually"})
	wcData.ValidPermute = append(wcData.ValidPermute, SelectData{"1", "Permute all"})
	wcData.ValidPermute = append(wcData.ValidPermute, SelectData{"2", "Run all tests at once"})
	wcData.ValidHosts = append(wcData.ValidHosts, SelectData{"10.10.101.7", "CSOS c4.xlarge"})
	wcData.ValidHosts = append(wcData.ValidHosts, SelectData{"10.10.101.38", "c4.xlarge"})
	wcData.ValidHosts = append(wcData.ValidHosts, SelectData{"10.10.203.148", "EC2 c4.xlarge"})
	wcData.ValidHosts = append(wcData.ValidHosts, SelectData{"10.10.102.5", "CSOS c5.18xlarge"})
	wcData.CSOptions = append(wcData.CSOptions, SelectData{"--aggressive", "Aggressive"})
	wcData.CSOptions = append(wcData.CSOptions, SelectData{"--minimize", "Minimize"})
	wcData.CSOptions = append(wcData.CSOptions, SelectData{"--maximize", "Maximize"})
	wcData.CSOptions = append(wcData.CSOptions, SelectData{"--timer-slack", "Timer Slack Mode"})
	wcData.CSOptions = append(wcData.CSOptions, SelectData{"--ignite-cpu", "Ignite (run CPU hot) Mode"})
}

func main() {
	var myYaml twsref.TestWSYaml
	pts.PTLogger().Info("httpserver started")
	defer pts.PTLogger().CloseLog()
	yamlfilename := ProcessCmdLine()

	if !twsref.LoadTWSYaml(*yamlfilename, &myYaml) {
		panic("Issue with YAML config file.")
	}
	myTestWS := NewWebServer(&myYaml)
	// Temporary function call to fill in data for server, tests, etc
	FillHCData(myTestWS)
	log.Printf("%v\n", myTestWS.ValidTests)
	myTestWS.SetupWebServer()
	myTestWS.Run()
}
