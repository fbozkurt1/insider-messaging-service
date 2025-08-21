create table messages
(
    id                     bigserial
        primary key,
    content                varchar(255)                           not null,
    recipient_phone_number varchar(15)                            not null,
    sending_status         varchar(10)                            not null,
    retry_count            integer                  default 0     not null,
    created_at             timestamp with time zone default now() not null,
    updated_at             timestamp with time zone default now() not null
);

create index idx_messages_sending_status on messages(sending_status);
create index idx_messages_sending_status_retry_count on messages(sending_status, retry_count);

