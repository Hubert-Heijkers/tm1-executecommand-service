# Execute Command Service

The "ExecuteCommand" service is a very simple tool that allows clients familiar with the `ExecuteCommand` TI function from TM1 v11 to execute a command. This can be done either by waiting for the process to finish or by returning immediately, similar to how the `ExecuteCommand` TI function operates.

## The Service

This service supports an "ExecuteCommand" resource which can be invoked either as a function using a GET request or as an action using a POST request. When called as a function, the `CommandLine` and `Wait` parameters are passed as query options. When called as an action, the request body must contain a JSON object with two properties: `CommandLine` and `Wait`, representing a string and an integer, respectively.

### Example Requests

#### With Wait: 1 (wait for the command to complete)

As a function:

```bash
curl "http://localhost:8080/execute?CommandLine=/path/to/script.sh+arg1+arg2&Wait=1"
```

Or as an action:

```bash
curl -X POST http://localhost:8080/ExecuteCommand \
    -d '{"CommandLine":"/path/to/script.sh arg1 arg2", "Wait": 1}' \
    -H "Content-Type: application/json"
```

Both requests execute `/bin/bash /path/to/script.sh arg1 arg2` and wait for the script to complete before returning the output.

#### With Wait: 0 (return immediately after starting the command)

As a function:

```bash
curl "http://localhost:8080/execute?CommandLine=/path/to/script.sh+arg1+arg2&Wait=0"
```

Or as an action:

```bash
curl -X POST http://localhost:8080/ExecuteCommand \
    -d '{"CommandLine":"/path/to/script.sh arg1 arg2", "Wait": 0}' \
    -H "Content-Type: application/json"
```

Either of these requests will start `/bin/bash /path/to/script.sh arg1 arg2` and return immediately with the message "Command started successfully" without waiting for the script to finish.

### URL Encoding
- When calling the service as a function using GET, remember to properly URL-encode your query parameters, especially the `CommandLine` string, which may contain spaces or special characters.
- For example, you can use `+` to represent spaces between arguments, or use `%20` for a literal space.

### How it works

- When called as a function using a GET request, the service extracts the `CommandLine` and `Wait` parameters from the URL query string.
- When called as an action using a POST request, the `CommandLine` and `Wait` parameters are extracted from the JSON object in the request body.
- The `Wait` parameter allows the client to specify whether to wait for the command to finish (`Wait: 1`) or to start the command and return immediately (`Wait: 0`).
- If `Wait: 0`, the command is executed asynchronously, meaning the HTTP response is returned right after starting the command, without waiting for the process to complete.
- If `Wait: 1`, the server waits for the command to finish and sends the output (or any errors) back to the client.

> Note that while TM1's `ExecuteCommand` function didn't return anything, if the requester is willing to wait for the command to complete, the output is returned in the response body, ready to be consumed if needed.

### How to use

Run the Go application directly from source, optionally specifying a port using the --port flag, as in:

```bash
go run main.go --port=9090
```

This will start the ExecuteCommand service listening to port `9090`. If no port is specified, it will default to port `8080`.

Once you are satisfied with the service, or want to use it as is, you would first build an executable using:

```bash
go build
./tm1-executecommand-service.exe --port=9090
```

This places the executable in the root of the source directory, or:

```bash
go install
tm1-executecommand-service.exe --port=9090
```

This also builds the executable but instead of cluttering your source directory, it places it in the `bin` folder of your workspace defined by the `GOPATH` environment variable. This snippet assumes that the `bin` folder is in your path, thus the OS will know where to find your executable for the ExecuteCommand service.

### Installing the ExecuteCommand service

The code checks if it is running as a Windows service and will act accordingly. To set up the ExecuteCommand service as a Windows service, create/register the service and start it with sc.exe:

```bash
sc.exe create TM1-ExecuteCommand-Service binPath= "C:\path\to\your\tm1-executecommand-service.exe"
sc.exe start TM1-ExecuteCommand-Service
```

## Migrating from TM1 v11 to v12

While this service could potentially be generally useful, the trigger for creating it was that TM1 v12 no longer supports the `ExecuteCommand` TI function. The main reason for this removal is that TM1 v12, presumed to be running in a container, would not allow you to execute commands in that limited context, nor would SREs of a SaaS offering or your IT team managing your cluster want you to.

TM1 v12 introduces an `ExecuteHttpRequest` function which, in essence, provides you with even greater power as long as the capability you are looking for is available as a 'service' and is accessible to you through HTTP[S]. As numerous `ExecuteCommand` examples popped up that only needed a context they could run in, and had absolutely no dependency on anything TM1 itself, the idea was born to create a lightweight service to provide such context in which they could continue to be executed and make migration of those `ExecuteCommand` requests straightforward.

This ExecuteCommand service is that service that can help the migration/transitioning to TM1 v12. The only thing required is a straightforward, almost search and replace, conversion of calls to `ExecuteCommand` to calls to `ExecuteHttpRequest` instead. Whilst considering using this pattern keep in mind there are a couple of limitations:

- There is no access to the data directory in a v12 deployment any longer (and you shouldn't if you happen to be running v12 standalone/locally), apart from the fact that the (meta-)data is organized completely differently anyway, so any commands you are trying to execute that require access and depend on those files/structures to be there will need to be rewritten.
- If the command you are executing utilizes applications/utilities, like `TM1RunTI`, that have been built using the older TM1 APIs then those will need to be rewritten as well as TM1 v12 only supports the OData compliant REST API.
- TM1's ExecuteCommand would implicitly look in the data directory of your TM1 server, as well as in the folder which held your `tm1s[d].exe`, for the command you were trying to execute, however, the ExecuteCommand service obviously does not. While there is no notion of either directory any longer, other than adding folders to the `PATH`, you can still start processes in the working directory of your ExecuteCommand service if you prepend the command to be executed with `.\` if your ExecuteCommand service is running on Windows or `./` in the case of Linux.

Provided none of the aforementioned dependencies exist, any existing `ExecuteCommand` requests can easily be replaced by an `ExecuteHttpRequest` function call.

For example:

```
ExecuteCommand( 'cmd /C echo %PATH%', 1 );
```

can simply be replaced by a request to the ExecuteCommand service:

```
ExecuteHttpRequest( 'GET', 'http://<<host>>:<<port>>/ExecuteCommand?CommandLine=cmd+/C+echo+%25PATH%25&Wait=1' );
```

or by calling the ExecuteCommand service as an action as in:

```tm1-ti
ExecuteHttpRequest( 'POST', 
                    'http://<<host>>:<<port>>/ExecuteCommand', 
                    '-h Content-Type:application/json',
                    '-d { "CommandLine":"cmd /C echo %PATH%", "Wait":1 }' );
```

> Note that either method will result in the exact same outcome, and that the command line itself needs to be URL-encoded in the case it is included in a query option as part of the URL request but does not need to be when injecting it in the JSON body of the POST request.
