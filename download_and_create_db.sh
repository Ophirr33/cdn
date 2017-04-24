wget http://geolite.maxmind.com/download/geoip/database/GeoLiteCity_CSV/GeoLiteCity-latest.tar.xz &&
  tar xf GeoLiteCity-latest.tar.xz &&
  mv GeoLiteCity_20170404/*.csv . &&
  sqlite3 locations.db < create_locations_db.sql 2> /dev/null &&
  rm *.csv &&
  rm -r GeoLite*
