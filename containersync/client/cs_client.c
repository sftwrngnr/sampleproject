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


int GetShmidBlock(key_t key, int shmsize) {
    int retval = shmget(key, shmsize, 0);
    if ( retval > 0 ) {
        Gshmblk = shmat(retval, NULL, 0);
        if (Gshmblk == (char *) -1 ) {
            printf("Failed to attach to shared memory with key %i id of %d\n", key, retval);
        }
    }
    return retval;
}

void DetachFromSharedMemoryBlock(int shmid) {
    printf("DetachFromSharedMemoryBlock\n");
    shmdt(Gshmblk);
}

    
void WaitForSignalFromServer(int shmid, int shmOffset) {
    int cursVal = -1;
    int curcnt = 0;
    int s;
    int Done=0;
    sem_t *mySem;
    struct timespec ts;
    int offset = shmOffset * sizeof(sem_t);
    mySem = (sem_t *) Gshmblk + offset;
    printf("WaitForSignalFromServer\n"); 
    while (!Done) {
        if (clock_gettime(CLOCK_REALTIME, &ts) == -1) {
         printf("Failed to get clock time.\n");
         DetachFromSharedMemoryBlock(shmid);
         _exit(EXIT_FAILURE);
        }
        ts.tv_sec += 1;
        s = sem_timedwait(mySem, &ts);
        if (s == -1 && errno == ETIMEDOUT ) {
            printf("Timed wait timed out. Current count is %i\n", curcnt);
        } else if (s != -1) {
            printf("Received DONE signal!\n");
            Done = 1;
        }
    }
}

void SignalWaitingServer(int shmOffset) {
    sem_t *mySem;
    int offset = shmOffset * sizeof(sem_t);
    mySem = (sem_t *) Gshmblk + offset;
    //memcpy(mySem, Gshmblk + offset, sizeof(sem_t));
    if (sem_post(mySem) != 0) {
        printf("Blew chow attempting to post semaphore.\n");
    }
    printf("Posted to server.\n");
}


int
main(int argc, char ** argv) {
    int shmid = 0;
    

    shmid = GetShmidBlock(SHM_SEG_ID, SHM_BLOCK_SIZE);
    if (shmid == -1) {
        printf("Failed to retrieve shared memoryid %x resulted in errno %i\n", SHM_SEG_ID, errno);
        return -1;
    }

    SignalWaitingServer(CNT_SEM_IDX); // Signal server
    WaitForSignalFromServer(shmid, SYNC_SEM_IDX); // Wait for server
    DetachFromSharedMemoryBlock(shmid);
    return 0;
}


