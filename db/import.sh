#!/bin/bash

mysql -uroot --local-infile enjoyotel <<EOS
load data local infile '/docker-entrypoint-initdb.d/init.csv' into table enjoyotel.livers fields terminated by ',' enclosed by '"' ignore 1 lines (name, @debuted_on, @retired_on) set debuted_on = date(@debuted_on), retired_on = date(@retired_on);
EOS
