package main

import (
	"log"
	"strings"
	"syscall"

	"github.com/prometheus/procfs"
	pts "github.com/sftwrngnr/sampleproject/pkg"
)

func CheckForRunningProcs(procnme []string) []int {
	var cklist []int
	procs, err := procfs.AllProcs()
	if err != nil {
		panic("Error getting all procs.")
	}
	for _, myProcNme := range procnme {
		for _, myProc := range procs {
			cmdLine, cErr := myProc.CmdLine()
			if cErr != nil {
				log.Printf("Error getting command line. %s\n", cErr)
				continue
			}
			if len(cmdLine) > 0 {
				if strings.Contains(cmdLine[0], myProcNme) {
					cklist = append(cklist, myProc.PID)
				}
				if len(cmdLine) > 1 {
					for _, ck := range cmdLine {
						if strings.Contains(ck, myProcNme) {
							cklist = append(cklist, myProc.PID)
						}
					}
				}
			}
		}
	}
	return cklist
}

func KillProcs(inprocs []int) {
	log.Printf("Killing processes %v\n", inprocs)
	for _, pid := range inprocs {
		syscall.Kill(pid, syscall.SIGKILL)
	}
}

func CheckPidFile() int {
	return -1
}

func WritePidFile() {
}

// We're going to use golang's native rpc stuff
func SetupRPCServer(myyaml pts.TestProgMgrYaml) {
	var srvrs []pts.SrvConfig
	// Hard coded for testing
	for _, progItem := range myyaml.RunList {
		var srvcfg pts.SrvConfig
		srvcfg.RunPath = progItem.RunPath
		srvcfg.RunCmd = progItem.RunProg
		srvcfg.ListenAddress = progItem.Host
		srvcfg.SType = pts.RpcServerType(progItem.SrvType)
		srvrs = append(srvrs, srvcfg)
	}
	pts.StartPTRpcServer(srvrs)

}

func main() {
	// Support for yaml config
	var myYaml pts.TestProgMgrYaml
	pts.PTLogger().Info("testprogmgr started.")
	defer pts.PTLogger().CloseLog()
	if !pts.LoadTestProcMgrConfigYamlFile(&myYaml, "./testprocmgr.yaml") {
		log.Fatal("Configuration yaml file testprocmgr.yaml not found.")
	}
	//log.Printf("%v\n", myYaml)
	proclist := make([]string, len(myYaml.RunList))
	for idx, tcmd := range myYaml.RunList {
		proclist[idx] = tcmd.Name
	}
	log.Printf("Checking for active testrunner processes.\n")
	pList := CheckForRunningProcs(proclist)
	if len(pList) > 0 {
		// Check to see if we've got a pid file in /tmp
		oldPid := CheckPidFile()
		if oldPid > -1 {
			pList = append(pList, oldPid)
		}
		KillProcs(pList)
	}
	// Write pid file
	WritePidFile()
	// Set up Shared memory
	// run Forrest RUN!!
	SetupRPCServer(myYaml)

}
