-- PostgreSQL setup script for the search engine database.
-- Adjusted for better performance and best practices.

-- To run this setup script manually from a PostgreSQL UI:
-- Define the following variables in psql replacing their values
-- with your own and then run the script.
--\set POSTGRES_DB 'SitesIndex'
--\set CROWLER_DB_USER 'your_username'
--\set CROWLER_DB_PASSWORD 'your_password'

-- Sources table stores the URLs or the information's seed to be crawled
CREATE TABLE IF NOT EXISTS Sources (
    source_id SERIAL PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP,
    url TEXT NOT NULL UNIQUE,         -- Using TEXT for long URLs
    status VARCHAR(50) DEFAULT 'new', -- All new sources are set to 'new' by default
    last_crawled_at TIMESTAMP,
    last_error TEXT,                  -- Using TEXT for potentially long error messages
    last_error_at TIMESTAMP,
    restricted INTEGER DEFAULT 2,     -- 0 = fully restricted (just this URL)
                                      -- 1 = l3 domain restricted (everything within this URL l3 domain)
                                      -- 2 = l2 domain restricted
                                      -- 3 = l1 domain restricted
                                      -- 4 = no restrictions
    disabled BOOLEAN DEFAULT FALSE,
    flags INTEGER DEFAULT 0,
    config JSONB                      -- Stores JSON document with all details about the source
                                      -- configuration for the crawler
);

-- Owners table stores the information about the owners of the sources
CREATE TABLE IF NOT EXISTS Owners (
    owner_id SERIAL PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    details JSONB NOT NULL             -- Stores JSON document with all details about the owner
);

-- NetInfo table stores the network information retrieved from the sources
CREATE TABLE IF NOT EXISTS NetInfo (
    netinfo_id SERIAL PRIMARY KEY,
    source_id INT REFERENCES Sources(source_id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    details JSONB NOT NULL
);

-- SearchIndex table stores the indexed information from the sources
CREATE TABLE IF NOT EXISTS SearchIndex (
    index_id SERIAL PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    page_url TEXT NOT NULL UNIQUE,                  -- Using TEXT for long URLs
    title VARCHAR(255),
    summary TEXT NOT NULL,                          -- Assuming summary is always required
    snapshot_url TEXT,                              -- Using TEXT for long URLs
    detected_type VARCHAR(8),                       -- (content type) denormalized for fast searches
    detected_lang VARCHAR(8),                       -- (URI language) denormalized for fast searches
    content TEXT
);

-- MetaTags table stores the meta tags from the SearchIndex
CREATE TABLE IF NOT EXISTS MetaTags (
    metatag_id SERIAL PRIMARY KEY,
    index_id INTEGER REFERENCES SearchIndex(index_id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    name VARCHAR(255),
    content TEXT
);

-- Keywords table stores all the found keywords during an indexing
CREATE TABLE IF NOT EXISTS Keywords (
    keyword_id SERIAL PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    keyword VARCHAR(100) NOT NULL UNIQUE
);

-- SourceOwner table stores the relationship between sources and their owners
CREATE TABLE IF NOT EXISTS SourceOwner (
    source_owner_id SERIAL PRIMARY KEY,
    source_id INTEGER NOT NULL,
    owner_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_source
        FOREIGN KEY(source_id)
        REFERENCES Sources(source_id)
        ON DELETE CASCADE,
    CONSTRAINT fk_owner
        FOREIGN KEY(owner_id)
        REFERENCES Owners(owner_id)
        ON DELETE CASCADE,
    UNIQUE(source_id, owner_id) -- Ensures unique combinations of source_id and owner_id
);

-- SourceSearchIndex table stores the relationship between sources and the indexed pages
CREATE TABLE IF NOT EXISTS SourceSearchIndex (
    ss_index_id SERIAL PRIMARY KEY,
    source_id INTEGER NOT NULL,
    index_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_source
        FOREIGN KEY(source_id)
        REFERENCES Sources(source_id)
        ON DELETE CASCADE,
    CONSTRAINT fk_search_index
        FOREIGN KEY(index_id)
        REFERENCES SearchIndex(index_id)
        ON DELETE CASCADE,
    UNIQUE(source_id, index_id) -- Ensures unique combinations of source_id and index_id
);

-- KeywordIndex table stores the relationship between keywords and the indexed pages
CREATE TABLE IF NOT EXISTS KeywordIndex (
    keyword_index_id SERIAL PRIMARY KEY,
    keyword_id INTEGER REFERENCES Keywords(keyword_id),
    index_id INTEGER REFERENCES SearchIndex(index_id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    occurrences INTEGER
);

-- Creates an index for the Sources url column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_sources_url') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_sources_url ON Sources(url text_pattern_ops);
    END IF;
END
$$;

-- Creates an index for the Sources status column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_sources_status') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_sources_status ON Sources(status);
    END IF;
END
$$;

-- Creates an index for the Sources last_crawled_at column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_sources_last_crawled_at') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_sources_last_crawled_at ON Sources(last_crawled_at);
    END IF;
END
$$;

-- Creates an index for the Owners details column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_owners_details') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_owners_details ON Owners USING gin(details jsonb_path_ops);
    END IF;
END
$$;

-- Creates an index for SourceOwner owner_id column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_sourceowner_owner_id') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX IF NOT EXISTS idx_sourceowner_owner_id ON SourceOwner(owner_id);
    END IF;
END
$$;

-- Creates an index for SourceOwner source_id column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_sourceowner_source_id') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX IF NOT EXISTS idx_sourceowner_source_id ON SourceOwner(source_id);
    END IF;
END
$$;


-- Creates an index for the source_id column in the NetInfo table
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_netinfo_source_id') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_netinfo_source_id ON NetInfo(source_id);
    END IF;
END
$$;

-- Creates an index for the scanned_on column in the NetInfo table
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_netinfo_last_updated_at') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_netinfo_last_updated_at ON NetInfo(last_updated_at);
    END IF;
END
$$;

-- Creates an index for the report column in the NetInfo table
-- This index is used to search for specific keys in the JSONB column
-- The jsonb_path_ops operator class is used to index the JSONB column
-- for queries that use the @> operator to search for keys in the JSONB column
-- The jsonb_path_ops operator class is optimized for queries that use the @> operator
-- to search for keys in the JSONB column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_json_netinfo_details') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_json_netinfo_details ON NetInfo USING gin (details jsonb_path_ops);
    END IF;
END
$$;

-- Creates an index for the Sources source_id column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_sources_source_id') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_sources_source_id ON Sources(source_id);
    END IF;
END
$$;

-- Creates an index for the SourceSearchIndex source_id column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_ssi_source_id') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_ssi_source_id ON SourceSearchIndex(source_id);
    END IF;
END
$$;

-- Creates an index for the SourceSearchIndex index_id column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_ssi_index_id') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_ssi_index_id ON SourceSearchIndex(index_id);
    END IF;
END
$$;

-- Creates an index for the SearchIndex title column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_searchindex_title') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_searchindex_title ON SearchIndex(title text_pattern_ops) WHERE title IS NOT NULL;
    END IF;
END
$$;

-- Creates an index for the SearchIndex summary column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_searchindex_summary') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_searchindex_summary ON SearchIndex(left(summary, 1000) text_pattern_ops) WHERE summary IS NOT NULL;
    END IF;
END
$$;

-- Creates an index for the SearchIndex content column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_searchindex_content') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_searchindex_content ON SearchIndex(left(content, 1000) text_pattern_ops) WHERE content IS NOT NULL;
    END IF;
END
$$;

-- Creates an index for the SearchIndex snapshot_url column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_searchindex_snapshot_url') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_searchindex_snapshot_url ON SearchIndex(snapshot_url) WHERE snapshot_url IS NOT NULL;
    END IF;
END
$$;

-- Creates an index for the SearchIndex last_updated_at column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_searchindex_last_updated_at') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_searchindex_last_updated_at ON SearchIndex(last_updated_at);
    END IF;
END
$$;

-- Creates an index for the MetaTags index_id column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_metatags_index_id') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_metatags_index_id ON MetaTags(index_id);
    END IF;
END
$$;

-- Creates an index for the MetaTags name column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_metatags_name') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_metatags_name ON MetaTags(name text_pattern_ops) WHERE name IS NOT NULL;
    END IF;
END
$$;

-- Creates and index for the Keywords ocurences column to help
-- with keyowrds analysis
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_keywordindex_occurrences') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_keywordindex_occurrences ON KeywordIndex(occurrences);
    END IF;
END
$$;

-- Adds a tsvector column for full-text search
DO $$
BEGIN
    -- Check and add the content_fts column if it does not exist
    IF NOT EXISTS (
        SELECT
            1
        FROM
            information_schema.columns
        WHERE
            table_name = 'searchindex' AND
            column_name = 'content_fts'
    ) THEN
        ALTER TABLE SearchIndex ADD COLUMN content_fts tsvector;
    END IF;
END
$$;

-- Creates an index on the tsvector column
DO $$
BEGIN
    -- Check if the index already exists
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_searchindex_content_fts') THEN
        -- Create the index if it doesn't exist
        CREATE INDEX idx_searchindex_content_fts ON SearchIndex USING gin(content_fts);
    END IF;
END
$$;

-- Creates a function to update the tsvector column
CREATE OR REPLACE FUNCTION searchindex_content_trigger() RETURNS trigger AS $$
BEGIN
  NEW.content_fts := to_tsvector('english', coalesce(NEW.content, ''));
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

-- Creates a trigger to update the tsvector column
DO $$
BEGIN
    -- Check if the trigger already exists
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_searchindex_content') THEN
        -- Create the trigger if it doesn't exist
        CREATE TRIGGER trg_searchindex_content BEFORE INSERT OR UPDATE
        ON SearchIndex FOR EACH ROW EXECUTE FUNCTION searchindex_content_trigger();
    END IF;
END
$$;

-- Creates a function to update the last_updated_at column
CREATE OR REPLACE FUNCTION update_last_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Creates a trigger to update the last_updated_at column
DO $$
BEGIN
    -- Check if the trigger already exists
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_update_sources_last_updated_before_update') THEN
		CREATE TRIGGER trg_update_sources_last_updated_before_update
		BEFORE UPDATE ON Sources
		FOR EACH ROW
		EXECUTE FUNCTION update_last_updated_at_column();
	END IF;
END
$$;

-- Creates a function to fetch and update the sources as an atomic operation
-- this is required to be able to deploy multiple crawlers without the risk of
-- fetching the same source multiple times
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'update_sources') THEN
        DROP FUNCTION update_sources(INTEGER);
    END IF;
END
$$;
CREATE OR REPLACE FUNCTION update_sources(limit_val INTEGER)
RETURNS TABLE(source_id INT, url TEXT, restricted INT, flags INT, config JSONB, last_updated_at TIMESTAMP) AS
$$
BEGIN
    RETURN QUERY
    WITH SelectedSources AS (
        SELECT s.source_id
        FROM Sources AS s
        WHERE s.disabled = FALSE
          AND (
               (s.last_updated_at IS NULL OR s.last_updated_at < NOW() - INTERVAL '3 days')
            OR (s.status = 'error' AND s.last_updated_at < NOW() - INTERVAL '15 minutes')
            OR (s.status = 'completed' AND s.last_updated_at < NOW() - INTERVAL '1 week')
            OR s.status = 'pending' OR s.status = 'new' OR s.status IS NULL
          )
        FOR UPDATE
        LIMIT limit_val
    )
    UPDATE Sources
        SET status = 'processing'
    WHERE Sources.source_id IN (SELECT SelectedSources.source_id FROM SelectedSources)
    RETURNING Sources.source_id, Sources.url, Sources.restricted, Sources.flags, Sources.config, Sources.last_updated_at;
END;
$$
LANGUAGE plpgsql;

--------------------------

-- Creates a new user
CREATE USER :CROWLER_DB_USER WITH ENCRYPTED PASSWORD :'CROWLER_DB_PASSWORD';

-- Grants permissions to the user on the :"POSTGRES_DB" database
GRANT CONNECT ON DATABASE :"POSTGRES_DB" TO :CROWLER_DB_USER;
GRANT USAGE ON SCHEMA public TO :CROWLER_DB_USER;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO :CROWLER_DB_USER;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO :CROWLER_DB_USER;
ALTER ROLE :CROWLER_DB_USER SET search_path TO public;
