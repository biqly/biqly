#!/usr/bin/env ruby
# frozen_string_literal: true

require 'csv'

# AdventureWorks for Postgres — derived from lorint/AdventureWorks-for-Postgres
# with fixes for:
# - UTF-8 vs UTF-16 LE/BE from Microsoft zip (upstream always used UTF-16LE).
# - Pipe rows skipped by `break if !is_needed` and never written (`if is_needed` only).
# - Pipe rows with `"`, tabs, or NUL inside a field must be COPY-CSV–quoted (Person XML,
#   Document bytea, etc.).
# - Leading NUL on some UTF-8 exports (e.g. Document.csv).

def postgres_copy_csv_field(s)
  # SQL Server CSV uses a single NUL for empty varbinary/bytea; Postgres COPY needs \N.
  return '\N' if s == "\x00"

  if /["\n\r\t\0]/.match?(s)
    '"' + s.gsub('"', '""') + '"'
  else
    s
  end
end

def load_csv_text(path)
  if path.end_with?('/Address.csv')
    return File.binread(path).force_encoding('Windows-1252').encode('UTF-8')
  end

  raw = File.binread(path).dup.force_encoding(Encoding::BINARY)
  raw = raw.delete_prefix("\xEF\xBB\xBF".b)
  raw = raw.delete_prefix("\x00".b)
  if raw.start_with?("\xFF\xFE".b)
    return raw.byteslice(2..).force_encoding('UTF-16LE').encode('UTF-8')
  end
  if raw.start_with?("\xFE\xFF".b)
    return raw.byteslice(2..).force_encoding('UTF-16BE').encode('UTF-8')
  end

  u8 = raw.dup.force_encoding('UTF-8')
  return u8 if u8.valid_encoding?

  raw.force_encoding('UTF-16LE').encode('UTF-8')
end

Dir.glob('./*.csv') do |csv_file|
  begin
    text = load_csv_text(csv_file)
  rescue Encoding::InvalidByteSequenceError, Encoding::UndefinedConversionError
    next
  end

  is_address = csv_file.end_with?('/Address.csv')
  is_needed = is_address
  output = +''
  pipe_buf = +''
  is_first = true
  is_pipes = false

  text.each_line do |line|
    if is_first
      is_pipes = true if line.include?('+|')
      if line[0] == "\uFEFF"
        line = line[1..]
        is_needed = true
      end
    end
    is_first = false

    if is_pipes
      if line.strip.end_with?('&|')
        pipe_buf << line.gsub('|474946383961', '|\\\\x474946383961')
                        .strip.delete_suffix('&|')
        output << pipe_buf.split('+|').map { |part| postgres_copy_csv_field(part) }.join("\t")
        output << "\n"
        pipe_buf = +''
      else
        pipe_buf << line.gsub("\r\n", '\\n')
      end
    else
      chomped = line.chomp("\r\n").gsub("\tE6100000010C", "\t\\\\xE6100000010C")
      row = CSV.parse_line(chomped, col_sep: "\t", liberal_parsing: true, quote_char: "\x00")
      if row
        output << row.map { |cell| postgres_copy_csv_field(cell) }.join("\t")
        output << "\n"
      end
    end
  end

  next if output.empty?

  puts "Processing #{csv_file}"
  File.write(csv_file + '.xyz', output)
  File.delete(csv_file)
  File.rename(csv_file + '.xyz', csv_file)
end
