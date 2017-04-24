CREATE TABLE locations(locId integer primary key, country text, region text, city text, postalCode text, latitude real, longitude real, metroCode text, areaCode text);
CREATE TABLE blocks(startIpNum integer, endIpNum integer, locId integer, foreign key(locId) references locations(locId));

.separator ,
.import GeoLiteCity-Blocks.csv blocks
.import GeoLiteCity-Location.csv locations

