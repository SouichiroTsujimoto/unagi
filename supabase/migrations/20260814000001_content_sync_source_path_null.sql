drop index if exists articles_source_path_idx;

create unique index if not exists articles_source_path_idx
  on articles(source_path)
  where source_path is not null and source_path <> '';
