# psoftjmx
Custom library to support extracting metric data from PeopleSoft instances.

This library is designed to read a list of PeopleSoft targets/instances and read metric data from the JMX source for web, app, and scheduler servers.

The library uses a custom [JMXQuery](https://github.com/UMN-PeopleSoft/JMXQuery) tool that can natively connect to Weblogic domains and PeopleSoft's Tuxedo app/scheduler server rmi services.

To ensure optimal performance of reading the metric data, the java JMXQuery service is ran as a Nailgun server to keeps JVM instances in memory for reuse.
Java must be used to get JMX data since the special module libraries to read Weblogic metrics is only written in Java.  This library abstracts the management of the [Nailgun server](https://github.com/facebook/nailgun) and handling of the JMX Queries.  A custom [NailGun Client](https://github.com/UMN-PeopleSoft/nailgo) was redesigned to work with the Nailgun Server.

This library was written in Go, specifically to support extracting the metric data to an ELK Metricbeat.  A separate repository defines the Metricbeat layer that leverages this library.

My first go program from scratch, so probably doesn't follow good idiomatic Go code, but it will eventually get there.
   
