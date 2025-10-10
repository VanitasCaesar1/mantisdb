// File Sync Operations
use std::fs::File;
use std::io;

pub fn sync_file(file: &File) -> io::Result<()> {
    file.sync_all()
}

pub fn sync_data(file: &File) -> io::Result<()> {
    file.sync_data()
}
