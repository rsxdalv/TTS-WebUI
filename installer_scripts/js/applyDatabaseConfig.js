const fs = require("fs");
const { resolve } = require("path");
const { displayMessage } = require("./displayMessage.js");

async function applyDatabaseConfig() {
  const db_conf = {
    location: resolve(__dirname, "..", "..", "data", "sqlite"),
    database: "webui.db",
  };

  const initializeDatabase = async () => {
    if (!fs.existsSync(db_conf.location)) {
      displayMessage("Creating SQLite database directory...");
      fs.mkdirSync(db_conf.location, { recursive: true });
      displayMessage("Successfully created SQLite database directory");
    } else {
      displayMessage("SQLite database directory already exists, skipping...");
    }
  };

  await initializeDatabase();

  // SQLite doesn't require start/stop - it's file-based
  const withDatabase = async (func) => {
    displayMessage("Using SQLite database...");
    try {
      await func();
    } catch (error) {
      throw error;
    }
  };

  const db_version = "1";
  const db_version_file = resolve(__dirname, "..", ".db_version");

  const withDatabaseVersioning = async (func) => {
    if (fs.existsSync(db_version_file)) {
      const version = fs.readFileSync(db_version_file, "utf8");
      if (version === db_version) {
        displayMessage(
          `Database is already up to date with version=${version}, skipping...`
        );
        return;
      } else {
        displayMessage(
          `Database is not up to date, current version=${version}, version=${db_version}, upgrading...`
        );
        await func();
        fs.writeFileSync(db_version_file, db_version);
      }
    }
  };

  await withDatabaseVersioning(() =>
    withDatabase(async () => {
      const dbPath = resolve(db_conf.location, db_conf.database);
      displayMessage(`Using SQLite database at ${dbPath}`);
      // SQLite database will be created automatically when first accessed
      const applyMigrations = async () => {
        const sql = async (strings) => {
          // Since we're assuming no interpolated values, we can directly use the string
          const query = strings[0];
          const psqlCommand = `psql -U ${db_conf.username} -d ${
            db_conf.database
          } -c "${query.replace(/"/g, '\\"').replace(/\n/g, " ")}"`;
          await $sh(psqlCommand);
        };

        await sql`CREATE TABLE IF NOT EXISTS generations (
      id SERIAL PRIMARY KEY,
      metadata JSONB
    );`;
      };
      await createDB();
      await applyMigrations();
    })
  );
}

exports.applyDatabaseConfig = applyDatabaseConfig;
