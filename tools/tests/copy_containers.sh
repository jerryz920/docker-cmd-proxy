#!/bin/bash

target=${1:-dead}
id=${2:-`uuidgen | tr -d -`}
tooldir=`readlink -f .`
srcdir=$tooldir/../../tests/$target
containerdir=$tooldir/../../tests/containers/$id
cp -r $srcdir $containerdir



