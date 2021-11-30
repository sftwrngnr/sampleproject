#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <time.h>
#include <errno.h>
#include <assert.h>
#include <unistd.h>
#include <sys/ipc.h>
#include <sys/shm.h>
#include <fcntl.h>
#include <semaphore.h>

#include "../include/cs_sync.h"

char *Gshmblk;

sem_t * CreateSemaphore(key_t shmid, char *SemName, int offst_ref ) {
    sem_t *retval = NULL;
    sem_t *mySem ;
    uint32_t offst = offst_ref * sizeof(sem_t);
    printf("CreateSemaphore %i, %s\n", shmid, SemName);
    if (offst > SHM_BLOCK_SIZE) {
        printf("Exceeded shared memory block size.\n");
    } else {
        mySem = sem_open(SemName, O_CREAT , 0777 );
        if (mySem != SEM_FAILED) {
            printf("Woohoo! Created semaphore.\n");
            memcpy(Gshmblk + offst, mySem, sizeof(sem_t));
            
            retval = (sem_t *) Gshmblk + offst;
        } else {
            printf("Fuck chocolate shakes.\n");
        }
    }
    return retval;
}


int CreateShmid(key_t key, int shmsize) {
    int retval = shmget(key, shmsize, IPC_CREAT | 0666);
    if ( retval > 0 ) {
        Gshmblk = shmat(retval, NULL, 0);
        if (Gshmblk == (char *) -1 ) {
            printf("Failed to attach to shared memory with key %i id of %d\n", key, retval);
        }
    }
    return retval;
}

void ReleaseSharedMemoryBlock(int shmid) {
    struct shmid_ds myshmid_block;
    int retval;
    printf("ReleaseSharedMemoryBlock\n");
    if (Gshmblk == (char *) -1) {
        printf("Could not access shared memory block.\n");
        return;
    }
    sem_unlink(CNT_SEM_NAME);
    sem_unlink(SYNC_SEM_NAME);
    //shmdt(Gshmblk);
    retval = shmctl(shmid, IPC_RMID, &myshmid_block);
    if (retval == -1) {
        printf("Problem removing shared memory block. shmid is %i. errno is %i\n", shmid, errno);
    }
}

    
int WaitForAllProcs(int shmid, sem_t *inSem, int wCount, int printWait) {
    int cursVal = -1;
    int curcnt = 0;
    int s;
    int maxloop = 20;
    int retval = -1;
    struct timespec ts;
    printf("WaitForAllProcs count is %i\n", wCount);
    while ((curcnt < wCount) && (maxloop > 0)) {
        if (clock_gettime(CLOCK_REALTIME, &ts) == -1) {
         printf("Failed to get clock time.\n");
         ReleaseSharedMemoryBlock(shmid);
         _exit(EXIT_FAILURE);
        }
        ts.tv_sec += 5;
        s = sem_timedwait(inSem, &ts);
        if (s == -1 && errno == ETIMEDOUT ) {
            // Semaphore was decremented... not by us or this is the first time
            // through
            if (printWait) {
                printf("Timed wait timed out. Current count is %i\n", curcnt);
                maxloop--;
            }
        } else if (s != -1) {
            printf("Received signal.\n");
            curcnt++;
        }
    }
    if (curcnt >= wCount) {
      retval=1;
      printf("All processes have checked in.\n");
    }
    return retval;
}

void SignalWaitingProcs(sem_t *inSem, int wCount, int printWait) {
    printf("SignalWaitingProcs \n");
    for (int nCount = 0; nCount < wCount; nCount++) {
        sem_post(inSem);
    }
}


int
main(int argc, char ** argv) {
    int shmid = 0;
    sem_t *CntSem, *SyncSem;
    key_t shmkey;
    int wcount = 0;
    
    if (argc != 2) {
        printf("Usage:\n cs_server NN\nWhere NN is the number of containers to wait for.\n");
        return -1;
    }
    wcount = atoi(argv[1]);
    if (wcount < 1) {
        printf("Container count must be >= 1 %s\n", argv[1]);
        return -1;
    }
    printf("wcount is %i\n", wcount);

    if (fork() == 0) {
    // We're now going to fork, as shared memory and semaphores have been created
   shmid = CreateShmid(SHM_SEG_ID, SHM_BLOCK_SIZE);
   if (shmid == -1) {
       printf("Failed to create shared memoryid %x resulted in errno %i\n", SHM_SEG_ID, errno);
       return -1;
    }

        CntSem = CreateSemaphore(shmid, CNT_SEM_NAME, CNT_SEM_IDX);
        if (CntSem == NULL) {
            printf("Failed to create semaphore %s errno is %i\n", CNT_SEM_NAME, errno);
            ReleaseSharedMemoryBlock(shmid);
            return -1;
        }

        SyncSem = CreateSemaphore(shmid, SYNC_SEM_NAME, SYNC_SEM_IDX );
        if (SyncSem == NULL) {
            printf("Failed to create semaphore %s\n", SYNC_SEM_NAME);
            ReleaseSharedMemoryBlock(shmid);
            return -1;
        }
        /* We're going to redirect stderr and stdout to /tmp/cs_server.out */
        int out = open("/tmp/cs_server.out", O_CREAT, 0666);
        if (-1 == out) { perror("opening /tmp/cs_server.out"); return 255; }

        int err = open("/tmp/cs_server.errlog", O_CREAT, 0666);
        if (-1 == err) { perror("opening /tmp/cs_server.errlog"); return 255; }
        int save_out = dup(fileno(stdout));
        int save_err = dup(fileno(stderr));
        if (-1 == dup2(out, fileno(stdout))) { perror("cannot redirect stdout"); return 255; }
        if (-1 == dup2(err, fileno(stderr))) { perror("cannot redirect stderr"); return 255; }
        if (1 == WaitForAllProcs(shmid, CntSem, wcount, 1) ) {// Wait for all clients to report
	    // Wait for things to quiesce then signal
	    sleep(5);
            SignalWaitingProcs(SyncSem, wcount, 1); // Signal all clients to start
            sleep(5); // Correct way to do this, is to check shared memory for attached processes.
        }
        ReleaseSharedMemoryBlock(shmid);
        fflush(stdout); close(out);
        fflush(stderr); close(err);

        dup2(save_out, fileno(stdout));
        dup2(save_err, fileno(stderr));

        close(save_out);
        close(save_err);
  } else {
    printf("Parent proc is exiting.\n");
    sleep(5);
  }
    return 0;
}


