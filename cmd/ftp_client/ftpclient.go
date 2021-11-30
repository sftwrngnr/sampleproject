package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	pts "github.com/sftwrngnr/sampleproject/pkg"
)

var sigDone = make(chan bool, 1)

type FTPServer struct {
	Host            string
	Port            int
	User            string
	Password        string
	Sshcert         string
	SSHCertPass     string
	Sshprivatekey   string
	SSHPubKey       string
	RetrieveDataLoc string
	TargetDataLoc   string
}

func NewFTPServer(yamlref *pts.FTPClientConfig) *FTPServer {

	return &FTPServer{
		Host:            yamlref.Host,
		Port:            yamlref.Port,
		User:            yamlref.User,
		Password:        yamlref.Password,
		Sshcert:         yamlref.Sshcert,
		SSHCertPass:     yamlref.SSHCertPass,
		Sshprivatekey:   yamlref.Sshprivatekey,
		SSHPubKey:       yamlref.SSHPubKey,
		RetrieveDataLoc: yamlref.RetrieveDataLoc,
		TargetDataLoc:   yamlref.TargetDataLoc,
	}
}

func MyCallBack(ftpdata pts.FTPProgData) {
	log.Printf("TotFiles %d, CurFileNum %d, Message %s\n", ftpdata.TotalFiles, ftpdata.CurFile, ftpdata.CurMsg)
	var req pts.PTRpcSrvReq
	retstat := new(pts.ReturnStatus)
	req.ReqCmd = pts.SetStatus
	req.State = pts.RetrievingDataFiles
	req.CurrentStep = ftpdata.CurFile
	req.TotalSteps = ftpdata.TotalFiles
	req.StatusMsg = make([]string, 1)
	req.StatusMsg = append(req.StatusMsg, ftpdata.CurMsg)
	pts.RpcClientRequest(&req, "127.0.0.1", retstat)
}

func TransferComplete() {
	var req pts.PTRpcSrvReq
	retstat := new(pts.ReturnStatus)
	req.ReqCmd = pts.SetStatus
	req.State = pts.RetrievingDataFilesComplete
	req.CurrentStep = 0
	req.TotalSteps = 0
	req.StatusMsg = make([]string, 1)
	req.StatusMsg = append(req.StatusMsg, "Retrieving data files complete.")
	pts.RpcClientRequest(&req, "127.0.0.1", retstat)
}

func (tws *FTPServer) GetTargetDir(RunNum int) string {
	retval := tws.TargetDataLoc
	if strings.Contains(tws.TargetDataLoc, "<RUN>") {
		retval = fmt.Sprintf("%srun%d", tws.TargetDataLoc[0:strings.Index(tws.TargetDataLoc, "<RUN>")], RunNum)
	}

	return retval
}

func (tws *FTPServer) SendDataPath(mydp string) {
	log.Printf("SendDataPath")
	var req pts.PTRpcSrvReq
	retstat := new(pts.ReturnStatus)
	req.ReqCmd = pts.SetData
	req.State = pts.Data
	req.CurrentStep = 0
	req.TotalSteps = 0
	req.StatusMsg = append(req.StatusMsg, "DataPath")
	req.StatusMsg = append(req.StatusMsg, mydp)
	pts.RpcClientRequest(&req, "127.0.0.1", retstat)
}

func (tws *FTPServer) Run(RunNum int) {
	// Begin by attempting the sftp connection with the parameters that we have
	if sshFtp := pts.NewSFTPWrap(tws.User, tws.Password, tws.Host, tws.Port, tws.Sshcert, tws.SSHCertPass, tws.Sshprivatekey, tws.SSHPubKey); sshFtp != nil {
		if tws.RetrieveDataLoc == "" {
			log.Printf("Retrieve data location is nil. Cannot retrieve any data")
		}
		if tws.TargetDataLoc == "" {
			log.Printf("Target data location is nil. Cannot retrieve any data")
		}
		sshFtp.Callback = MyCallBack
		myTargetDir := tws.GetTargetDir(RunNum)
		tws.SendDataPath(myTargetDir)
		sshFtp.TransferFilesFromServer(tws.RetrieveDataLoc, myTargetDir)
		TransferComplete()
	} else {
		log.Printf("File transfer failed.\n")
	}

}

func (tws *FTPServer) RetrieveStatus(w http.ResponseWriter, r *http.Request) {
	// This will return status info for use with the test page.
	r.ParseForm()
	var req pts.PTRpcSrvReq
	retstat := new(pts.ReturnStatus)
	req.ReqCmd = pts.GetStatus
	retstat = new(pts.ReturnStatus)
	pts.RpcClientRequest(&req, "127.0.0.1", retstat)
	outstr, err := json.MarshalIndent(retstat, "", "    ")
	if err != nil {
		fmt.Fprintf(w, "Error unmarshalling status data.")
	} else {
		//enc := json.NewEncoder(w)
		//fmt.Fprintf(w, "<div stat-data='")
		//enc.Encode(retstat)
		fmt.Fprintf(w, string(outstr))
		//fmt.Fprintf(w, "'></div>")
	}

}

func main() {
	var myYaml pts.FTPClientConfig
	pts.PTLogger().Info("FTPClient started.")
	defer pts.PTLogger().CloseLog()

	if !pts.LoadFTPConfigYamlFile("./ftpclient.yaml", &myYaml) {
		panic("Issue with YAML config file.")
	}
	myFTPClient := NewFTPServer(&myYaml)
	myFTPClient.Run(myYaml.CurRun)
	// Post increment and write
	if myYaml.IncRun {
		myYaml.CurRun++
		pts.WriteFTPConfigYamlFile("./ftpclient.yaml", &myYaml)
	}
}
