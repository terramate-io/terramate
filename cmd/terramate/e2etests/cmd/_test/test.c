#define _POSIX_C_SOURCE  200809L
#include <stdlib.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <signal.h>
#include <string.h>
#include <stdio.h>
#include <errno.h>

static inline const char *signal_name(const int signum)
{
    switch (signum) {
    case SIGINT:  return "interrupt";
    case SIGHUP:  return "SIGHUP";
    case SIGTERM: return "SIGTERM";
    case SIGQUIT: return "SIGQUIT";
    case SIGUSR1: return "SIGUSR1";
    case SIGUSR2: return "SIGUSR2";
    default:      return "(unnamed)";
    }    
}

int main(void)
{
    sigset_t  mask;
    siginfo_t info;
    int       signum;    

    sigemptyset(&mask);
    sigaddset(&mask, SIGINT);
    if (sigprocmask(SIG_BLOCK, &mask, NULL) == -1) {
        fprintf(stderr, "Cannot block SIGUSR1: %s.\n", strerror(errno));
        return EXIT_FAILURE;
    }

    printf("ready\n");
    fflush(stdout);

    while (1) {
        signum = sigwaitinfo(&mask, &info);
        if (signum == -1) {

            /* If some other signal was delivered to a handler installed
               without SA_RESTART in sigaction flags, it will interrupt
               slow calls like sigwaitinfo() with EINTR error. So, those
               are not really errors. */
            if (errno == EINTR)
                continue;

            printf("Parent process: sigwaitinfo() failed: %s.\n", strerror(errno));
            return EXIT_FAILURE;
        }

        printf("%s\n", signal_name(signum));
        fflush(stdout);
    }

    printf("Done.\n");
    return EXIT_SUCCESS;
}