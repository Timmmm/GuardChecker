# GuardChecker

This is a very very very simple and hacky program for checking whether C/C++ header
files have C++ include guards like this:

	#ifdef __cplusplus
	extern "C" {
	#endif

and

	#ifdef __cplusplus
	}
	#endif
	
It uses some very sketchy regexes to find those lines, and then if they are not present
it tries to insert them. It uses further sketchy regexes to find the normal header
include guards and inserts the `__cplusplus` ones inside them.

It modifies files without asking so I highly recommend you only use it on code
tracked by version control. It's also pretty hacky so I suggest you at least skim the results!

Usage is trivial:

    ./GuardChecker <path>

It will check all files ending in `.h`