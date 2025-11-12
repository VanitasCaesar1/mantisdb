//! Geospatial Database
//!
//! Geospatial data types and queries for location-based applications

use crate::error::{Error, Result};
use parking_lot::RwLock;
use std::collections::HashMap;
use std::sync::Arc;
use serde::{Serialize, Deserialize};

/// Geospatial database
pub struct GeospatialDB {
    inner: Arc<RwLock<GeospatialInner>>,
}

struct GeospatialInner {
    collections: HashMap<String, GeoCollection>,
}

/// Geospatial collection
struct GeoCollection {
    name: String,
    features: Vec<GeoFeature>,
}

/// Geospatial feature
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GeoFeature {
    pub id: String,
    pub geometry: Geometry,
    pub properties: HashMap<String, serde_json::Value>,
}

/// Geometry types
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum Geometry {
    Point(Point),
    LineString(LineString),
    Polygon(Polygon),
    MultiPoint(MultiPoint),
    MultiLineString(MultiLineString),
    MultiPolygon(MultiPolygon),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Point {
    pub coordinates: [f64; 2], // [longitude, latitude]
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LineString {
    pub coordinates: Vec<[f64; 2]>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Polygon {
    pub coordinates: Vec<Vec<[f64; 2]>>, // First vec is outer ring, rest are holes
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MultiPoint {
    pub coordinates: Vec<[f64; 2]>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MultiLineString {
    pub coordinates: Vec<Vec<[f64; 2]>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MultiPolygon {
    pub coordinates: Vec<Vec<Vec<[f64; 2]>>>,
}

impl GeospatialDB {
    /// Create a new geospatial database
    pub fn new() -> Self {
        Self {
            inner: Arc::new(RwLock::new(GeospatialInner {
                collections: HashMap::new(),
            })),
        }
    }
    
    /// Create a collection
    pub fn create_collection(&self, name: String) -> Result<()> {
        let mut inner = self.inner.write();
        
        if inner.collections.contains_key(&name) {
            return Err(Error::General(format!("Collection '{}' already exists", name)));
        }
        
        inner.collections.insert(name.clone(), GeoCollection {
            name,
            features: Vec::new(),
        });
        
        Ok(())
    }
    
    /// Insert a feature
    pub fn insert(&self, collection: &str, feature: GeoFeature) -> Result<()> {
        let mut inner = self.inner.write();
        
        let coll = inner.collections.get_mut(collection)
            .ok_or_else(|| Error::General(format!("Collection '{}' not found", collection)))?;
        
        coll.features.push(feature);
        Ok(())
    }
    
    /// Find features near a point
    pub fn nearby(
        &self,
        collection: &str,
        center: Point,
        radius_meters: f64,
        limit: Option<usize>,
    ) -> Result<Vec<GeoFeature>> {
        let inner = self.inner.read();
        
        let coll = inner.collections.get(collection)
            .ok_or_else(|| Error::General(format!("Collection '{}' not found", collection)))?;
        
        let mut results: Vec<_> = coll.features.iter()
            .filter_map(|feature| {
                if let Geometry::Point(point) = &feature.geometry {
                    let distance = haversine_distance(
                        center.coordinates[0],
                        center.coordinates[1],
                        point.coordinates[0],
                        point.coordinates[1],
                    );
                    
                    if distance <= radius_meters {
                        Some((feature.clone(), distance))
                    } else {
                        None
                    }
                } else {
                    None
                }
            })
            .collect();
        
        // Sort by distance
        results.sort_by(|a, b| a.1.partial_cmp(&b.1).unwrap());
        
        let results: Vec<_> = results
            .into_iter()
            .take(limit.unwrap_or(usize::MAX))
            .map(|(feature, _)| feature)
            .collect();
        
        Ok(results)
    }
    
    /// Check if a point is within a polygon
    pub fn within(
        &self,
        collection: &str,
        point: Point,
    ) -> Result<Vec<GeoFeature>> {
        let inner = self.inner.read();
        
        let coll = inner.collections.get(collection)
            .ok_or_else(|| Error::General(format!("Collection '{}' not found", collection)))?;
        
        let results: Vec<_> = coll.features.iter()
            .filter(|feature| {
                if let Geometry::Polygon(polygon) = &feature.geometry {
                    point_in_polygon(&point.coordinates, &polygon.coordinates[0])
                } else {
                    false
                }
            })
            .cloned()
            .collect();
        
        Ok(results)
    }
    
    /// Get all features in a bounding box
    pub fn bbox(
        &self,
        collection: &str,
        min_lon: f64,
        min_lat: f64,
        max_lon: f64,
        max_lat: f64,
    ) -> Result<Vec<GeoFeature>> {
        let inner = self.inner.read();
        
        let coll = inner.collections.get(collection)
            .ok_or_else(|| Error::General(format!("Collection '{}' not found", collection)))?;
        
        let results: Vec<_> = coll.features.iter()
            .filter(|feature| {
                if let Geometry::Point(point) = &feature.geometry {
                    let lon = point.coordinates[0];
                    let lat = point.coordinates[1];
                    lon >= min_lon && lon <= max_lon && lat >= min_lat && lat <= max_lat
                } else {
                    false
                }
            })
            .cloned()
            .collect();
        
        Ok(results)
    }
    
    /// Calculate distance between two points
    pub fn distance(&self, point1: &Point, point2: &Point) -> f64 {
        haversine_distance(
            point1.coordinates[0],
            point1.coordinates[1],
            point2.coordinates[0],
            point2.coordinates[1],
        )
    }
}

impl Clone for GeospatialDB {
    fn clone(&self) -> Self {
        Self {
            inner: Arc::clone(&self.inner),
        }
    }
}

/// Calculate distance using Haversine formula (in meters)
fn haversine_distance(lon1: f64, lat1: f64, lon2: f64, lat2: f64) -> f64 {
    const EARTH_RADIUS: f64 = 6371000.0; // meters
    
    let lat1_rad = lat1.to_radians();
    let lat2_rad = lat2.to_radians();
    let delta_lat = (lat2 - lat1).to_radians();
    let delta_lon = (lon2 - lon1).to_radians();
    
    let a = (delta_lat / 2.0).sin().powi(2)
        + lat1_rad.cos() * lat2_rad.cos() * (delta_lon / 2.0).sin().powi(2);
    
    let c = 2.0 * a.sqrt().atan2((1.0 - a).sqrt());
    
    EARTH_RADIUS * c
}

/// Ray casting algorithm for point-in-polygon test
fn point_in_polygon(point: &[f64; 2], polygon: &[[f64; 2]]) -> bool {
    let x = point[0];
    let y = point[1];
    let mut inside = false;
    
    let n = polygon.len();
    let mut j = n - 1;
    
    for i in 0..n {
        let xi = polygon[i][0];
        let yi = polygon[i][1];
        let xj = polygon[j][0];
        let yj = polygon[j][1];
        
        let intersect = ((yi > y) != (yj > y))
            && (x < (xj - xi) * (y - yi) / (yj - yi) + xi);
        
        if intersect {
            inside = !inside;
        }
        
        j = i;
    }
    
    inside
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_create_collection() {
        let db = GeospatialDB::new();
        let result = db.create_collection("places".to_string());
        assert!(result.is_ok());
    }
    
    #[test]
    fn test_insert_point() {
        let db = GeospatialDB::new();
        db.create_collection("places".to_string()).unwrap();
        
        let feature = GeoFeature {
            id: "1".to_string(),
            geometry: Geometry::Point(Point {
                coordinates: [-73.9857, 40.7484], // NYC
            }),
            properties: HashMap::new(),
        };
        
        let result = db.insert("places", feature);
        assert!(result.is_ok());
    }
    
    #[test]
    fn test_nearby() {
        let db = GeospatialDB::new();
        db.create_collection("places".to_string()).unwrap();
        
        // Insert NYC
        db.insert("places", GeoFeature {
            id: "nyc".to_string(),
            geometry: Geometry::Point(Point {
                coordinates: [-73.9857, 40.7484],
            }),
            properties: HashMap::new(),
        }).unwrap();
        
        // Insert LA (far away)
        db.insert("places", GeoFeature {
            id: "la".to_string(),
            geometry: Geometry::Point(Point {
                coordinates: [-118.2437, 34.0522],
            }),
            properties: HashMap::new(),
        }).unwrap();
        
        // Search near NYC
        let results = db.nearby(
            "places",
            Point { coordinates: [-73.9857, 40.7484] },
            10000.0, // 10km radius
            None,
        ).unwrap();
        
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "nyc");
    }
    
    #[test]
    fn test_haversine_distance() {
        // NYC to LA
        let distance = haversine_distance(-73.9857, 40.7484, -118.2437, 34.0522);
        // Should be approximately 3,944 km = 3,944,000 meters
        assert!(distance > 3_900_000.0 && distance < 4_000_000.0);
    }
    
    #[test]
    fn test_point_in_polygon() {
        let point = [-73.9857, 40.7484];
        let polygon = vec![
            [-74.0, 40.7],
            [-73.9, 40.7],
            [-73.9, 40.8],
            [-74.0, 40.8],
        ];
        
        assert!(point_in_polygon(&point, &polygon));
    }
}
