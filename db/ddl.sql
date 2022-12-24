create table livers (
  `liver_id` bigint auto_increment primary key,
  `name` varchar(255) not null unique key,
  `age` int
) engine=InnoDB default character set=utf8mb4;
