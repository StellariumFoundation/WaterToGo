use std::env;

fn main() {
    let args: Vec<String> = env::args().collect();
    println!("Hello from Rust!");
    println!("Args: {:?}", &args[1..]);
}

fn parse_number(s: &str) -> Result<i32, String> {
    s.parse::<i32>().map_err(|e| format!("Invalid number: {}", e))
}
