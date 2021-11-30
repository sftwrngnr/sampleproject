package TestWebServer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

type StressNGParams struct {
	Tests         string `json:"Tests"`
	PermuteMethod int    `json:"PermuteMethod"`
	TestSeq       string `json:"TestSeq"`
	StressOpts    string `json:"StressOpts"`
	TestSuffix    string `json:"TestSuffix"`
}

type CPMemResParams struct {
	MemLimit       string `json:"MemLimit"`
	MemRes         string `json"MemRes"`
	TotMemLimit    string `json"TotMemLimit"`
	KernelMemLimit string `json"KernelMemLimit"`
	Swappiness     string `json:"Swappiness"`
	DisableOOM     bool   `json:"DisableOOM"`
	HeirArchAcct   bool   `json:"HeirArchAcct"`
}

type CPUMgmtParams struct {
	SharesRatio string `json:"SharesRatio"`
	Quota       string `json:Quota"`
	Period      string `json:"Period"`
	RTRuntime   string `json:"RTRuntime"`
	RTPeriod    string `json:"RTPeriod"`
	CPUS        string `json:"CPUS"`
	MemNodes    string `json:"MemNodes"`
}

type PidMgmtParams struct {
	MaxPids string `json:"MaxPids"`
}

type HugePageMgmtParams struct {
	PageSize string `json:"PageSize"`
	Limit    string `json:"Limit"`
}

type NetworkPriorityParams struct {
	ClassId   string `json:"ClassId"`
	LNClassId string `json:"LNClassId"`
}

type TestSuite struct {
	StressNG    StressNGParams        `json:"StressNG"`
	CP_MemRes   CPMemResParams        `json:"CP_MemRes"`
	CPU_Mgmt    CPUMgmtParams         `json:"CPU_Mgmt"`
	PID_Mgmt    PidMgmtParams         `json:"PID_Mgmt"`
	HugePg_Mgmt HugePageMgmtParams    `json:"HugePg_Mgmt"`
	NetId_Pri   NetworkPriorityParams `json:"NetId_Pri"`
}

type TestDef struct {
	TestInstance  string    `json:"TestInstance"`
	AddMonitoring [3]bool   `json:"AddMonitoring"`
	TestDuration  int       `json:"TestDuration"`
	Tsuite1       TestSuite `json:"Tsuite1,omitempty"`
	Tsuite2       TestSuite `json:"Tsuite2,omitempty"`
	Tsuite3       TestSuite `json:"Tsuite3,omitempty"`
}

var SessionMarshal = func(v interface{}) (io.Reader, error) {
	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

var SessionUnMarshal = func(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}

type TWSSession struct {
	Sessdir string
}

type SessState int

const (
	UnInit SessState = iota
	TestDefined
	TestExecFileWritten
	TestExecFileWriteFail
	TestDefTransferStarted
	TestDefTransferComplete
	TestDefTransferSuccess
	TestDefTransferFailed
	TestStarted
	TestRunning
	TestComplete
	TestFailed
	FTPDownloadStarted
	FTPDownloadComplete
	FTPDownloadFailed
	UpdateReadmeStarted
	UpdateReadmeComplete
	UpdateReadmeFailed
	CSVExportStarted
	CSVExportComplete
	CSVExportFailed
	GithubCommitStarted
	GithubCommitComplete
	GithubCommitFailed
)

type TWSSessionData struct {
	CurrentState   SessState
	ErrorMsg       string
	LastAccess     time.Time
	TestDefinition TestDef
}

func NewSession(sessdir string) *TWSSession {
	retval := new(TWSSession)
	retval.Sessdir = sessdir
	return retval
}

func (tws *TWSSession) CheckValidSession(uuid string) bool {
	retval := true
	fname := fmt.Sprintf("%s/%s.sess", tws.Sessdir, uuid)
	if _, err := os.Stat(fname); os.IsExist(err) {
		log.Printf("Invalid session file or file doesn't exist:  %s\n", fname)
		retval = false
	} else if err != nil {
		log.Printf("%v\n", err)
		retval = false
	}
	return retval
}

func (tws *TWSSession) CreateNewSession(uuid string) bool {
	retval := true
	fname := tws.getfname(uuid)
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Printf(err.Error())
		retval = false
	}

	if err := f.Close(); err != nil {
		log.Printf(err.Error())
		retval = false
	}
	return retval
}

func (tws *TWSSession) getfname(uuid string) string {
	return fmt.Sprintf("%s/%s.sess", tws.Sessdir, uuid)
}

func (tws *TWSSession) AddSessionData(uuid string, twsData TWSSessionData) bool {
	// Overwrite the session file no matter what
	retval := true
	fname := tws.getfname(uuid)
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_CREATE, 0755)
	twsData.LastAccess = time.Now()
	if err != nil {
		log.Printf(err.Error())
		retval = false
	} else {
		defer f.Close()
		wBytes, err := SessionMarshal(twsData)
		if err != nil {
			retval = false
			log.Printf("Error with SessionMarshal. %s\n", err.Error())
		} else {
			_, err := io.Copy(f, wBytes)
			if err != nil {
				retval = false
				log.Printf("Error with writing bytes. %s\n", err.Error())
			}
		}
	}
	return retval
}

func (tws *TWSSession) ErrorMsg(uuid string, errorMsg string) bool {
	var myData TWSSessionData
	if tws.GetSessionData(uuid, myData) {
		myData.ErrorMsg = errorMsg
		return tws.AddSessionData(uuid, myData)
	}
	return false
}

func (tws *TWSSession) GetSessionData(uuid string, twsData TWSSessionData) bool {
	retval := false
	fname := tws.getfname(uuid)
	f, err := os.Open(fname)
	if err == nil {
		defer f.Close()
		err := SessionUnMarshal(f, &twsData)
		if err != nil {
			fmt.Printf("Blew chow in GetSessionData: %s\n", err.Error())
		}
		retval = err == nil
	}
	return retval
}

func (tws *TWSSession) RemoveSessionFile(uuid string) bool {
	retval := true
	fname := tws.getfname(uuid)
	err := os.Remove(fname)
	if err != nil {
		log.Printf("Error removing file %s\n", fname)
		retval = false
	}
	return retval
}
