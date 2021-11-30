This golang program will execute the httperf container experiment. The program
will automatically connect to the container ec2 instance, execute the container
shutdown and instance destroy script, then execute the container creation script
with the appropriate parameters for the test.

Once that has been accomplished, the application will then log in to the load
boxes for the httperf clients, configure them appropriately and execute the
httperf measurement.

Once httperf has finished on each of the load testing client machines, the
result files will be pulled down to the machine running this application and placed
into a repository for further processing.
