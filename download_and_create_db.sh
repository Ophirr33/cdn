wget http://geolite.maxmind.com/download/geoip/database/GeoLiteCity_CSV/GeoLiteCity-latest.tar.xz 2> /dev/null &&
  tar xf GeoLiteCity-latest.tar.xz &&
  tail -n +3 < GeoLiteCity_20170404/GeoLiteCity-Blocks.csv > GeoLiteCity-Blocks.csv &&
  tail -n +3 < GeoLiteCity_20170404/GeoLiteCity-Location.csv > GeoLiteCity-Location.csv &&
  sqlite3 locations.db < create_locations_db.sql 2> /dev/null
rm *.csv &&
rm -r GeoLite*
