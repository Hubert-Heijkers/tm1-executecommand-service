# Execute Command Service

This, VERY SIMPLE, 'ExecuteCommand' service, allows clients, familiar with the ExecuteCommand TI function, to execute a command line, optionally waiting for the executed process to finish or not, like the ExecuteCommand TI function would have in TM1 v11.

## The service

This service supports a ExecuteCommand resource which can be called as a function, using a GET request, or as an action, using POST. When called as a function the CommandLine and Wait parameters are passed as query options as part of the request whereas when called as an action the body of the request has to contain a JSON object with two properties, named CommandLine and Wait respectively, containing the values for these parameters, a string and an integer respectively.

### Example Requests

With Wait: 1 (wait for the command to complete) as a function:

```bash
curl "http://localhost:8080/execute?CommandLine=/path/to/script.sh+arg1+arg2&Wait=1"
```

or as an action:

```bash
curl -X POST http://localhost:8080/ExecuteComand \
    -d '{"CommandLine":"/path/to/script.sh arg1 arg2", "Wait": 1}' \
    -H "Content-Type: application/json"
```

Either of these requests will execute `/bin/bash /path/to/script.sh arg1 arg2` and wait for the script to complete before returning the output.

With Wait: 0 (return immediately after starting the command) as a function:

```bash
curl "http://localhost:8080/execute?CommandLine=/path/to/script.sh+arg1+arg2&Wait=0"
```

or as an action:

```bash
curl -X POST http://localhost:8080/ExecuteCommand \
    -d '{"CommandLine":"/path/to/script.sh arg1 arg2", "Wait": 0}' \
    -H "Content-Type: application/json"
```

Either of these requests will start `/bin/bash /path/to/script.sh arg1 arg2` and return immediately with the message `"Command started successfully"` without waiting for the script to finish.

### URL Encoding
- Remember, when calling the service as a function, using GET, to properly URL-encode your query parameters, especially the commandLine string, which may contain spaces or special characters.
- For example, you can use + to represent spaces between arguments, or use %20 for a literal space.

### How it works
- When called as a function, using a GET request, the service extracts the `CommandLine` and `Wait` parameters from the URL query string.
- When called as an action, using a POST request, the `Commandline` and `Wait` parameters are extracted from the request body which MUST contain a JSON object with properties representing these parameters.
- The `Wait` parameter allows the client to specify whether to wait for the command to finish (`Wait: 1`) or to start the command and return immediately (`Wait: 0`).
- If `Wait: 0`, the command is executed asynchronously, meaning the HTTP response is returned right after starting the command, without waiting for the process to complete.
- If `Wait: 1`, the server waits for the command to finish and sends the output (or any errors) back to the client.

> Note that whilst TM1's `ExecuteCommand` function didn't return anything, if the requester is willing to wait for the command to complete the output is being returned in the response body, ready to be consumed if need be.

## Migrating from TM1 v11 to v12

TM1 v12 no longer supports the `ExecuteCommand` TI function. This service can help the transition to TM1 v12 by providing a means to execute commands that don't depend on having access to the data directory and/or use other utilities, like TM1RunTI, that utilize the older TM1 APIs that no longer are supported by TM1 v12 either. Provided none of these dependencies exist, any existing `ExecuteCommand` requests can easily be replaced by an `ExecuteHttpRequest` function call.

For example:

```tm1-ti
ExecuteCommand( 'cmd /C echo %PATH%', 1 );
```

can, for example, simply be replaced by a request to the ExecuteCommand service:

```tm1=ti
ExecuteHttpRequest( 'GET', 'http://<<host>>:<<port>>/ExecuteCommand?CommandLine=cmd+/C+echo+%25PATH%25&Wait=1' );
```

or calling the ExecuteCommand service as an action as in:

```tm1-ti
ExecuteHttpRequest( 'POST', 
                    'http://<<host>>:<<port>>/ExecuteCommand', 
                    '-h Content-Type:application/json',
                    '-d { "CommandLine":"cmd /C echo %PATH%", "Wait":1 }' );
```

> Note that either will result in the exact same result and that the command line itself needs to be ULR encoded in the case we include it in a query option as part of the ULR request but don't need to do so when injecting it in the JSON body of the POST request.
