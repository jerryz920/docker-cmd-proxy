#!/bin/bash

target=${1:-dead}
id=${2:-`uuidgen | tr -d -`}
tooldir=`readlink -f .`
srcdir=$tooldir/../../tests/$target
containerdir=$tooldir/../../tests/containers/$id

mkdir $containerdir
for f in `ls $srcdir`; do
  cp $srcdir/$f $containerdir/tmpfile
  pushd .
  cd $containerdir
  rename 's/tmpfile/'$f'/' tmpfile
  popd
done
