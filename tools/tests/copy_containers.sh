#!/bin/bash

target=${1:-dead}
id=${2:-`uuidgen | tr -d -`}
toolexec=`readlink -f $0`
tooldir=`dirname $toolexec`
srcdir=$tooldir/../../tests/$target
containerdir=$tooldir/../../tests/containers/$id
cp -r $srcdir $containerdir



