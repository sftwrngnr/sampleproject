package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	pts "github.com/sftwrngnr/sampleproject/pkg"
)

func ExecCSVCommand(execCmd string, params []string) {
	// Build command
	cmd := exec.Command("/usr/bin/go", "run", fmt.Sprintf("%s/%s", params[0], execCmd), params[1], params[2])
	cmd.Dir = params[0]
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	cmd.Wait()

}

func ExecGitCommands(execCmd string, dirLoc string, execLoc string) {
	log.Printf("ExecGitCommands dirLoc is %s %s\n", dirLoc, execLoc)
	cmd := exec.Command(execCmd, "add", dirLoc)
	cmd.Dir = dirLoc
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	cmd.Wait()
	ccmd := exec.Command(execCmd, "commit", "-a", "-m", "Automated commit of test results")
	ccmd.Dir = dirLoc
	cerr := ccmd.Start()
	if cerr != nil {
		log.Fatal(cerr)
	}
	ccmd.Wait()
	pcmd := exec.Command(execCmd, "push")
	pcmd.Dir = dirLoc
	perr := pcmd.Start()
	if perr != nil {
		log.Fatal(perr)
	}
	pcmd.Wait()
}

func main() {
	pts.PTLogger().Info("csvandcommit started.")
	defer pts.PTLogger().CloseLog()

	var progArgs string = "progArgs"
	var csvExec string = "csvExec"
	var csvDir string = "csvDir"
	if len(os.Args) > 2 {
		progArgs = os.Args[1]
		csvDir = os.Args[2]
		csvExec = os.Args[3]
	}
	log.Printf("csvandcommit %s %s\n", csvDir, csvExec)
	// First execute csv command
	cmdParams := make([]string, 3)
	cmdParams[0] = csvDir
	cmdParams[1] = fmt.Sprintf("-path=%s", progArgs)
	cmdParams[2] = "-n"
	ExecCSVCommand(csvExec, cmdParams)
	// Then execute github commit. Note.. this is ghetto. It'll use whatever credentials are on the server.
	var gitCmd string = "/usr/bin/git"
	ExecGitCommands(gitCmd, progArgs, "/usr/bin")
}
