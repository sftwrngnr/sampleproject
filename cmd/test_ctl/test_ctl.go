package main

import (
	"log"
	"time"

	pte "github.com/sftwrngnr/sampleproject/pkg"
)

//func GenPermute(permvals string[], int permType, int numAtaTime) string[] {
//}

func RunNginxTests(nt *pte.NginxTest) {
	nginxTestRuns := nt.ConfigureNginxTestParams()
	nt.NginxServerClearTickFiles()
	nt.TestPermute(nginxTestRuns)
	nt.ShutdownNginxServer()
}

func main() {
	pte.PTLogger().Info("test_ctl started.")
	defer pte.PTLogger().CloseLog()
	ntnull := (pte.NginxTest{})
	sngnull := &(pte.StressNGTest{})
	NTCfg := pte.InitNginxTest("./nginx_test.yaml")
	if NTCfg.ServerHostCfg != (ntnull.ServerHostCfg) {
		log.Printf("There were a total of %d hosts read\n", len(NTCfg.LoadHostCfg.Hosts))
		RunNginxTests(NTCfg)
	}
	NGSCfg := pte.InitStressNGTest("./stress_ng_test.yaml")
	if NGSCfg != sngnull {
		// Execute tests
		NGSCfg.TestPermute()
		NGSCfg.TestsCompleted()
		// Wait for 5 seconds to insure that message was sent
		myTimer := time.NewTimer(time.Second * 5)
		<-myTimer.C
	}

}
