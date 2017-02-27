#!/bin/bash

target=${1:-dead}
toolexec=`readlink -f $0`
tooldir=`dirname $toolexec`
srcdir=$tooldir/../../tests/$target
containerdir=$tooldir/../../tests/containers/
rm -r $containerdir/*



