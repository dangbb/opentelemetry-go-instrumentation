create table if not exists audits (
    id int unsigned not null auto_increment primary key,
    service_name varchar(55) not null,
    request_type int not null
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;