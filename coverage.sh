#!/bin/bash
echo "mode: count" > profile.cov;
for Dir in $(go list ./...);
do
  currDir=$(echo $Dir | sed 's/github.com\/G-Node\/gin-repo\///g');
  echo $currDir
  currCount=$(ls -l $currDir/*.go 2>/dev/null | wc -l);
  echo $currCount
  ls -l $currDir
  if [ $currCount -gt 0 ];
  then
    echo $Dir
    go test -covermode=count -coverprofile=tmp.out $Dir;
    if [ -f tmp.out ];
    then
      cat tmp.out | grep -v "mode: count" >> profile.cov;
    fi
  fi
done
rm tmp.out;
