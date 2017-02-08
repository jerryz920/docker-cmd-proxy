package api

/*
   Implements the internal API for all processes. To allow a container and related
   worker to access these APIs, docker should automatically map a volume containing
   the Unix domain socket of this API. The API is exposed using HTTP methods.
*/
