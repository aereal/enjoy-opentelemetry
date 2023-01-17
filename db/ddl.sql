drop table if exists livers;
create table if not exists livers (
  `liver_id` bigint auto_increment primary key,
  `name` varchar(255) not null unique key,
  `debuted_on` date not null,
  `retired_on` date
) engine=InnoDB default character set=utf8mb4;
