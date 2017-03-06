#!/bin/bash

if [ $# -eq 0 ]; then
  return 0
fi

toolexec=`readlink -f $0`
tooldir=`dirname $toolexec`
for id in $@; do
  if [[ x"$id" == x ]]; then
    continue
  fi 
  rm -r $tooldir/../../tests/containers/$id
done

