#!/usr/bin/env perl
use strict;
use warnings;

sub main {
  my $css = do { local $/; <STDIN> };
  $css =~ s{/\*.*?\*/}{}gs;
  $css =~ s{\@import[^;]*;}{}g;
  print filter_rules($css);
}

sub filter_rules {
  my ($text) = @_;
  my $result = '';
  while ($text =~ /\G\s*([^{}]+?)\s*\{/gc) {
    my $prelude = $1;
    my $body = read_body(\$text);
    $result .= keep_rule($prelude, $body);
  }
  return $result;
}

sub read_body {
  my ($text_ref) = @_;
  my $start = pos($$text_ref);
  my $depth = 1;
  my $index = $start;
  while ($depth > 0) {
    die "duvay-utility-filter: unbalanced braces\n" if $index >= length($$text_ref);
    my $character = substr($$text_ref, $index, 1);
    $depth += 1 if $character eq '{';
    $depth -= 1 if $character eq '}';
    $index += 1;
  }
  pos($$text_ref) = $index;
  return substr($$text_ref, $start, $index - $start - 1);
}

sub keep_rule {
  my ($prelude, $body) = @_;
  return keep_group($prelude, $body) if $prelude =~ /^\@(media|supports)\b/;
  return "$prelude {$body}\n" if $prelude =~ /^\@/;
  return keep_style_rule($prelude, $body);
}

sub keep_group {
  my ($prelude, $body) = @_;
  my $inner = filter_rules($body);
  return '' if $inner eq '';
  return "$prelude {\n$inner}\n";
}

sub keep_style_rule {
  my ($prelude, $body) = @_;
  my @kept = grep { !is_utility_selector($_) } split_selectors($prelude);
  return '' unless @kept;
  return join(', ', @kept) . " {$body}\n";
}

sub split_selectors {
  my ($prelude) = @_;
  my @selectors;
  my $current = '';
  my $depth = 0;
  for my $character (split //, $prelude) {
    $depth += 1 if $character eq '(';
    $depth -= 1 if $character eq ')';
    if ($character eq ',' && $depth == 0) {
      push @selectors, trim($current);
      $current = '';
      next;
    }
    $current .= $character;
  }
  push @selectors, trim($current);
  return @selectors;
}

sub is_utility_selector {
  my ($selector) = @_;
  my @classes = $selector =~ /\.(-?[A-Za-z_][\w-]*)/g;
  return scalar grep { is_utility_class($_) } @classes;
}

sub is_utility_class {
  my ($class) = @_;
  return 1 if $class !~ /^w-/;
  return $class =~ /^w-(\d+|auto)$/ ? 1 : 0;
}

sub trim {
  my ($text) = @_;
  $text =~ s/^\s+|\s+$//g;
  return $text;
}

main();
